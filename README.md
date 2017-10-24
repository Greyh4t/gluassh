# gluasocket


## example
```go
package main

import (
	"fmt"

	"github.com/Greyh4t/gluassh"
	"github.com/yuin/gopher-lua"
)

func main() {
	L := lua.NewState(
		lua.Options{
			CallStackSize: 512,
			RegistrySize:  512,
		},
	)
	L.PreloadModule("ssh", gluassh.Loader)
	err := L.DoString(
		`ssh=require("ssh")
		c=ssh.new()
		c:settimeout(5)
		err = c:connect("192.168.206.129", 22, "root", "password")
		if err == nil then
			stdout,stderr,err = c:exec("ping www.google.com", 5)
			c:close()
			print(stdout)
			print("---------------")
			print(stderr)
			print("---------------")
			print(err)
		else
			print(err)
		end`,
	)
	if err != nil {
		fmt.Println(err)
	}
}
```