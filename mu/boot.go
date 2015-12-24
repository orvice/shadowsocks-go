package main

import (
	"fmt"
	muconfig "github.com/orvice/shadowsocks-go/mu/config"
	"github.com/orvice/shadowsocks-go/mu/user"
	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
	"os"
	"strconv"
	"time"
)

var configFile string
var config *ss.Config

func boot() {
	var err error

	// log.SetOutput(os.Stdout)

	err = InitMySqlClient()
	if err != nil {
		Log.Error(err)
		os.Exit(0)
	}
	client := user.GetClient()
	users, err := client.GetUsers()
	if err != nil {
		Log.Error(err)
		os.Exit(0)
	}
	Log.Info(len(users))
	// clear storage
	// storage.ClearAll()
	bootUsers(users)
	time.Sleep(muconfig.Conf.Base.CheckTime * time.Second)

	go func() {
		for {
			// check users
			users, err = client.GetUsers()
			if err != nil {
				Log.Error(err)
				// os.Exit(0)
			}
			checkUsers(users)
			Log.Info("check finish...")
			time.Sleep(muconfig.Conf.Base.CheckTime * time.Second)
		}
	}()

	go func() {
		for {
			// check users
			users, err = client.GetUsers()
			if err != nil {
				Log.Error(err)
				// os.Exit(0)
			}
			syncUsers(users)
			Log.Info("sync finish...")
			time.Sleep(muconfig.Conf.Base.SyncTime * time.Second)
		}
	}()
	waitSignal()
}

// 第一次启动
func bootUsers(users []user.User) {
	for _, user := range users {
		Log.Info(user.GetUserInfo())
		err := storage.StoreUser(user.GetUserInfo())
		if err != nil {
			Log.Error(err)
		}
		go runWithCustomMethod(user)
	}
}

// check users
func checkUsers(users []user.User) {
	for _, user := range users {
		Log.Debug("check user for ", user.GetPort())

		isExists, err := storage.Exists(user)
		if err != nil {
			Log.Error("check exists error: ", err)
			continue
		}
		if !isExists && user.IsEnable() {
			Log.Info("new user to run", user)
			err := storage.StoreUser(user.GetUserInfo())
			if err != nil {
				Log.Error("store  error: ", err)
			}
			go runWithCustomMethod(user)
			continue
		}
		if !user.IsEnable() && isExists {
			Log.Info("user would be disable,port:  ", user.GetPort())
			passwdManager.del(strconv.Itoa(user.GetPort()))
			err := storage.Del(user)
			if err != nil {
				Log.Error(err)
			}
			continue
		}

		if !user.IsEnable() {
			continue
		}

		sUser, err := storage.GetUserInfo(user)
		if err != nil {
			Log.Error("get user error: ", err)
			continue
		}
		if sUser.Passwd != user.GetPasswd() || sUser.Method != user.GetMethod() {
			Log.Info(fmt.Sprintf("user port [%v] passwd or method change ,restart user...", user.GetPort()))
			passwdManager.del(strconv.Itoa(user.GetPort()))
			go runWithCustomMethod(user)
		}
	}
}

// sync users traffic
func syncUsers(users []user.User) {
	for _,user := range users {
		size,err := storage.GetSize(user)
		if err != nil{
			Log.Error(fmt.Sprintf("get size fail for port:%d",user.GetPort()),err)
			continue
		}
		err = user.UpdatetTraffic(int(size))
		if err != nil{
			Log.Error(fmt.Sprintf("update size fail for port:%d",user.GetPort()),err)
			continue
		}
		Log.Info(fmt.Sprintf("success update traffic usage for port %d,total update size %d",user.GetPort(),size))
		err = storage.SetSize(user,0)
		if err != nil{
			Log.Error(fmt.Sprintf("set storage size to 0 fail for port:%d",user.GetPort()),err)
			continue
		}
	}
}
