# Contributing

Issues and pull requests are welcome.

## Build and test

```sh
make            # tidy, fmt, vet, test, build -> bin/ssmhop
make test
make cover
```

No AWS account is needed. Discovery is written against narrow interfaces
(`ECSAPI`, `SSMLister`, `EC2Describer`) and the tests use fakes.

To exercise the picker by hand:

```sh
SSM_PICKER_DEMO=1 go test ./internal/picker -run Demo -v
```

## Pull requests

- keep `make` green
- add a test for anything that changes behaviour
- one concern per PR
- no real account data in code, tests or issue reports: no account IDs, instance
  IDs, ARNs or hostnames from a live account

## Layout

```
main.go               signal handling and exit codes
internal/cmd          cobra commands
internal/awsx         config and client construction
internal/target       discovery, the Target type, picker labels
internal/picker       fzf and built-in finders
internal/session      session-manager-plugin invocation
```
