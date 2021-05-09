package main

import (
	"fmt"
	"hash/crc32"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

// VerifyWsJwt check in ws
func VerifyJwt(jwtdata string, headers map[string][]string, sigkey string) (uid string, exp int64, err error) {
	t := time.Now()
	uid = ""
	exp = -1
	err = nil

	exp_sid := ""
	if headers != nil {
		agent := ""
		ipaddr := ""
		for name, hs := range headers {
			name = strings.ToLower(name)
			for _, h := range hs {
				if name == "user-agent" {
					agent += h
				} else if name == "x-real-ip" {
					ipaddr += h
				}
			}
		}
		exp_sid = fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(ipaddr+"/"+agent)))
	}

	signingKey := []byte(sigkey)
	accessToken, err := jwt.Parse(jwtdata, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return signingKey, nil
	})

	if err != nil {
		return
	}

	claims, ok := accessToken.Claims.(jwt.MapClaims)
	if ok && accessToken.Valid {
		if !claims.VerifyExpiresAt(t.UTC().Unix(), true) {
			err = fmt.Errorf("Token is expired")
			return
		}
	} else {
		err = fmt.Errorf("Token is not valid")
		return
	}

	// uid
	uidi, hasu := claims["uid"]
	if !hasu {
		err = fmt.Errorf("Token has no uid")
		return
	}
	// sid
	sidi, hass := claims["sid"]
	if !hass {
		err = fmt.Errorf("Token has no sid")
		return
	}
	// exp
	expi, hase := claims["exp"]
	if !hase {
		err = fmt.Errorf("Token has no exp")
		return
	}

	uid = ""
	switch uidv := uidi.(type) {
	case string:
		uid = uidv
	default:
		err = fmt.Errorf("Token is not valid")
		return
	}

	sid := ""
	switch sidv := sidi.(type) {
	case string:
		sid = sidv
	default:
		err = fmt.Errorf("Token is not valid")
		return
	}
	if headers != nil {
		if exp_sid != sid {
			err = fmt.Errorf("Token not valid")
			return
		}
	}

	exp = int64(0)
	switch expv := expi.(type) {
	case float64:
		exp = int64(expv)
	default:
		err = fmt.Errorf("Token is not valid")
		return
	}

	if len(uid) == 0 {
		err = fmt.Errorf("Token is not valid")
		return
	}

	return
}
