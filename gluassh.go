package gluassh

import (
	"bytes"
	"net"
	"strconv"
	"time"

	"github.com/yuin/gopher-lua"
	"golang.org/x/crypto/ssh"
)

type SSH struct {
	timeout time.Duration
	client  *ssh.Client
	session *ssh.Session
}

func newSSH(L *lua.LState) int {
	ud := L.NewUserData()
	ud.Value = &SSH{
		timeout: time.Second * 10,
	}
	L.SetMetatable(ud, L.GetTypeMetatable("ssh"))
	L.Push(ud)
	return 1
}

func Loader(L *lua.LState) int {
	luaSSH := L.NewTypeMetatable("ssh")
	L.SetGlobal("ssh", luaSSH)
	L.SetField(luaSSH, "new", L.NewFunction(newSSH))
	L.SetField(luaSSH, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"new":        newSSH,
		"settimeout": settimeout,
		"connect":    connect,
		"exec":       exec,
		"close":      close,
	}))
	L.Push(luaSSH)
	return 1
}

func checkSSH(L *lua.LState) *SSH {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*SSH); ok {
		return v
	}
	L.ArgError(1, "ssh.ssh expected")
	return nil
}

func settimeout(L *lua.LState) int {
	s := checkSSH(L)
	s.timeout = time.Duration(L.CheckInt(2)) * time.Second
	return 0
}

func connect(L *lua.LState) int {
	s := checkSSH(L)
	host := L.CheckString(2) + ":" + strconv.Itoa(L.CheckInt(3))
	username := L.CheckString(4)
	password := L.CheckString(5)
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		Timeout: s.timeout,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	s.client = client
	return 0
}

func exec(L *lua.LState) int {
	s := checkSSH(L)
	command := L.CheckString(2)
	session, err := s.client.NewSession()
	if err != nil {
		L.Push(lua.LString(""))
		L.Push(lua.LString(""))
		L.Push(lua.LString(err.Error()))
		return 3
	}
	defer session.Close()

	var o, e bytes.Buffer
	session.Stdout = &o
	session.Stderr = &e

	err = session.Run(command)

	L.Push(lua.LString(o.String()))
	L.Push(lua.LString(e.String()))

	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 3
	}
	return 2
}

func close(L *lua.LState) int {
	s := checkSSH(L)
	s.client.Close()
	if s.session != nil {
		s.session.Close()
	}
	return 0
}
