# Security

## Reporting

Report vulnerabilities privately through
[GitHub security advisories](https://github.com/tryoverflow/ssmhop/security/advisories/new).
Please don't open a public issue for a vulnerability.

## Scope

ssmhop holds no credentials of its own. It resolves them through the AWS SDK
default chain, calls `ssm:StartSession`, and hands the returned session ID,
stream URL and token to AWS's `session-manager-plugin` as process arguments,
the same way the AWS CLI does.

Worth knowing:

- session tokens are passed on the plugin's command line, so they are visible to
  anything that can read the process table on your machine for the life of the
  session
- sessions are logged by AWS against your identity, and `--reason` is recorded
  in the audit trail
- ssmhop writes no config, cache or credential files
