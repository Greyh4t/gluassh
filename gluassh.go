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
	luaSSH := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"new": newSSH,
	})

	mt := L.NewTypeMetatable("ssh")

	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"settimeout": settimeout,
		"connect":    connect,
		"exec":       exec,
		"close":      _close,
	}))
	L.SetField(luaSSH, "ssh", mt)

	L.Push(luaSSH)
	return 1
}

func AsyncLoader(L *lua.LState) int {
	luaSSH := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"new": newSSH,
	})

	mt := L.NewTypeMetatable("ssh")
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"settimeout": settimeout,
		"connect":    asyncConnect,
		"exec":       asyncExec,
		"close":      _close,
	}))
	L.SetField(luaSSH, "ssh", mt)

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

func _close(L *lua.LState) int {
	s := checkSSH(L)
	s.client.Close()
	return 0
}

//sync
func connect(L *lua.LState) int {
	s := checkSSH(L)
	host := L.CheckString(2) + ":" + strconv.Itoa(L.CheckInt(3))
	username := L.CheckString(4)
	password := L.CheckString(5)

	var sshConfig ssh.Config
	sshConfig.SetDefaults()
	sshConfig.Ciphers = append(sshConfig.Ciphers, "aes256-cbc", "aes128-cbc", "3des-cbc", "des-cbc")

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		Timeout: s.timeout,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Config: sshConfig,
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
	timeout := L.ToInt(3)
	session, err := s.client.NewSession()
	var o, e bytes.Buffer
	if err == nil {
		defer session.Close()
		session.Stdout = &o
		session.Stderr = &e
		err = session.Start(command)
		if err == nil {
			if timeout > 0 {
				timer := time.AfterFunc(time.Duration(timeout)*time.Second, func() {
					session.Close()
				})
				err = session.Wait()
				timer.Stop()
			} else {
				err = session.Wait()
			}
		}
	}
	L.Push(lua.LString(o.String()))
	L.Push(lua.LString(e.String()))
	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 3
	}
	return 2
}

//async
func asyncConnect(L *lua.LState) int {
	s := checkSSH(L)
	host := L.CheckString(2) + ":" + strconv.Itoa(L.CheckInt(3))
	username := L.CheckString(4)
	password := L.CheckString(5)
	resultChan := make(chan lua.LValue, 1)

	go func(s *SSH, host, username, password string, resultChan chan lua.LValue) {
		var sshConfig ssh.Config
		sshConfig.SetDefaults()
		sshConfig.Ciphers = append(sshConfig.Ciphers, "aes256-cbc", "aes128-cbc", "3des-cbc", "des-cbc")

		config := &ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			Timeout: s.timeout,
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			Config: sshConfig,
		}

		client, err := ssh.Dial("tcp", host, config)
		if err != nil {
			resultChan <- lua.LString(err.Error())
			close(resultChan)
			return
		}
		s.client = client
		close(resultChan)
	}(s, host, username, password, resultChan)

	return L.Yield(lua.LChannel(resultChan))
}

func asyncExec(L *lua.LState) int {
	s := checkSSH(L)
	command := L.CheckString(2)
	timeout := L.ToInt(3)
	resultChan := make(chan lua.LValue, 3)

	go func(s *SSH, command string, timeout int, resultChan chan lua.LValue) {
		session, err := s.client.NewSession()
		var o, e bytes.Buffer
		if err == nil {
			defer session.Close()
			session.Stdout = &o
			session.Stderr = &e
			err = session.Start(command)
			if err == nil {
				if timeout > 0 {
					timer := time.AfterFunc(time.Duration(timeout)*time.Second, func() {
						session.Close()
					})
					err = session.Wait()
					timer.Stop()
				} else {
					err = session.Wait()
				}
			}
		}
		resultChan <- lua.LString(o.String())
		resultChan <- lua.LString(e.String())
		if err != nil {
			resultChan <- lua.LString(err.Error())
		}
		close(resultChan)
	}(s, command, timeout, resultChan)

	return L.Yield(lua.LChannel(resultChan))
}
