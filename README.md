Reloader
==

Reload program when file changed


### Installation
```
go get github.com/dworld/reloader
```


### Usage

Create ``Reloader.yaml`` file in your working directory.

```yaml
watch:

- pattern: "*.rb"
  command: "./restart.sh"
  delay: 3000
  start: 1

- pattern: "*.yml"
  command: "./restart.sh"
  delay: 3000
  start: 0
```

Run ``reloader`` afterwards.
