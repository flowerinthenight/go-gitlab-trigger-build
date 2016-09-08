# go-gitlab-trigger-build

A simple tool to trigger a build in GitLab with an option to poll the build status until done.

# Examples

### Trigger a tagged build

```
go-gitlab-trigger-build -tag -ref master -url http://ip-addr/api/v3/projects/1/trigger/builds -version 1.0.1.1 -token [trigger-token] -usrtoken [private-token]
```

You can get your GitLab trigger token from Projects - Settings - Trigger option. The user's private token can be accessed from Profile Settings - Account option.

### Trigger a build for the `develop` branch without wait

```
go-gitlab-trigger-build -ref develop -url http://ip-addr/api/v3/projects/1/trigger/builds -token [trigger-token] -wait=false
```

# License

[The MIT License](./LICENSE.md)
