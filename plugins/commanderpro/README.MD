# commanderpro for Corsair Commander Pro 

Simply launch the main program to update the commander pro on itself.

```Bash
cd plugins/commanderpro && sudo go run . -c /opt/tmi
```


Run from termi after having installed the app:
`sudo $GOPATH/bin/commanderpro -c /opt/tmi`

`-c /opt/tmi` is optional since the default path is in fact `/opt/tmi`
