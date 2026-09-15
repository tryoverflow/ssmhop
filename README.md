# ssmhop

[![ci](https://github.com/tryoverflow/ssmhop/actions/workflows/ci.yml/badge.svg)](https://github.com/tryoverflow/ssmhop/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/tryoverflow/ssmhop?sort=semver)](https://github.com/tryoverflow/ssmhop/releases/latest)
[![go report](https://goreportcard.com/badge/github.com/tryoverflow/ssmhop)](https://goreportcard.com/report/github.com/tryoverflow/ssmhop)
[![license](https://img.shields.io/github/license/tryoverflow/ssmhop)](LICENSE)

Shells and port forwarding over AWS SSM, for EC2 instances **and** ECS containers.

No bastion, no inbound ports, no SSH keys, no pasting instance IDs. Run any
command bare and pick from a fuzzy list.

EC2 instances appear as `Name    (instance-id)` and ECS containers as
`cluster/service/container    (task-id)`, in one list. Anything that can't take
a session is tagged inline with the reason.

ECS Exec runs on SSM, so a container is addressable as
`ecs:<cluster>_<task-id>_<container-runtime-id>` and the AWS port-forwarding
documents work against it exactly as they do against an instance. That is why
`forward` covers both from one picker.

## Install

macOS, and it pulls in `session-manager-plugin` for you:

```sh
brew install --cask tryoverflow/tap/ssmhop
```

Anywhere else, or from source:

```sh
go install github.com/tryoverflow/ssmhop@latest
```

Going that route, AWS's `session-manager-plugin` has to be on your `PATH`
yourself. ssmhop shells out to it exactly the way the AWS CLI does:

```sh
brew install --cask session-manager-plugin
session-manager-plugin --version
```

On Apple Silicon check it's a native arm64 build (`lipo -archs $(which session-manager-plugin)`).
An x86_64 one fails with `bad CPU type in executable` without Rosetta 2.

`fzf` is optional: `brew install fzf`.

## Usage

```
ssmhop list      [filter]    list reachable EC2 instances and ECS containers
ssmhop connect   [filter]    shell on an EC2 instance
ssmhop exec      [filter]    shell or command in an ECS container
ssmhop forward   [filter]    forward a local port through an instance or container
ssmhop completion <shell>    bash | zsh | fish | powershell
ssmhop version
```

`filter` pre-narrows the picker. It matches case-insensitively on instance ID,
Name tag, cluster, service, container and task ID.

| Flag | |
| --- | --- |
| `-p`, `--profile` | AWS profile |
| `-r`, `--region` | AWS region |
| `-c`, `--cluster` | limit ECS discovery to clusters matching this substring |
| `--kind` | restrict to `ec2` or `ecs` |
| `--no-fzf` | use the built-in finder even if fzf is installed |

On busy accounts use `--cluster`. Without it, ECS discovery walks every cluster
and every running task.

### list

```sh
ssmhop list
ssmhop list --kind ecs --cluster prod
ssmhop list -o json | jq '.[] | select(.Kind == "ecs")'
```

The `READY` column says whether a session would connect, and why not when it
wouldn't. `-o json` always emits a list, never `null`.

### connect

```sh
ssmhop connect
ssmhop connect web-prod
ssmhop connect --reason "INC-1234"
```

### exec

```sh
ssmhop exec
ssmhop exec api --cluster prod
ssmhop exec api --command "ps aux"
```

### forward

A bare port forwards to a port on the target:

```sh
ssmhop forward --remote 8080
```

`host:port` forwards onward to anything the target can reach:

```sh
ssmhop forward --remote my-db.cluster-abc.eu-west-1.rds.amazonaws.com:5432 --local 15432
ssmhop forward api --kind ecs --cluster prod --remote 9000
```

`--local` defaults to the remote port. IPv6 literals need brackets:
`--remote '[fd00::1]:5432'`.

## The picker

fzf is used when it's on your `PATH`, because it can put the header and prompt
above the list (`--layout=reverse --header-first`). Without fzf the finder built
into the binary is used, which draws the prompt at the bottom. `--no-fzf` forces
the built-in one. A single candidate is taken without prompting.

To see it without an AWS account:

```sh
SSM_PICKER_DEMO=1 go test ./internal/picker -run Demo -v
SSM_PICKER_DEMO=1 SSM_PICKER_NOFZF=1 go test ./internal/picker -run Demo -v
```

## ECS Exec requirements

The task must have been started with `enableExecuteCommand`. This can't be
turned on for a running task; the service needs a new deployment:

```sh
aws ecs update-service --cluster CLUSTER --service SERVICE \
  --enable-execute-command --force-new-deployment
```

Also:

- the **task role** (not the execution role) can call `ssmmessages:CreateControlChannel`,
  `CreateDataChannel`, `OpenControlChannel`, `OpenDataChannel`
- Fargate platform version 1.4.0+, or ECS agent 1.50.2+ on EC2 launch type
- the container's `ExecuteCommandAgent` managed agent is `RUNNING`

`ssmhop list` reports which piece is missing.

## IAM

EC2:

```
ssm:DescribeInstanceInformation
ssm:StartSession        on the instances and documents you use
ssm:TerminateSession    on your own sessions
ec2:DescribeInstances   optional, only for Name tags
```

ECS:

```
ecs:ListClusters
ecs:ListTasks
ecs:DescribeTasks
ecs:ExecuteCommand
```

Missing permissions for one kind don't break the other. Discovery warns on
stderr and returns what it could reach, failing only if every kind failed.

## Notes

Ctrl-C goes to the remote shell, not to `ssmhop`. Terminal signals are ignored
while the plugin runs: it puts the TTY in raw mode and forwards `Ctrl-C` as a
byte, so handling `SIGINT` here would let the shell reclaim the terminal and the
plugin's stdin read would fail with `EIO`.

Cancelling the picker exits `130`.

## Development

```sh
make            # tidy, fmt, vet, test, build -> bin/ssmhop
make test
make lint
```

Discovery is written against narrow interfaces (`ECSAPI`, `SSMLister`,
`EC2Describer`), so the tests run with fakes and need no AWS account.

See [CONTRIBUTING.md](CONTRIBUTING.md). To report a vulnerability, see
[SECURITY.md](SECURITY.md).

## Prior art

[gossm](https://github.com/gjbae1212/gossm) and
[ssmctl](https://github.com/rhysmcneill/ssmctl) both do EC2 well; the picker layout here
is gossm's. Neither covers ECS, which is what ssmhop adds.

## License

MIT
