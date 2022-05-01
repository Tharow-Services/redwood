package main

import (
	"bufio"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// HTTP proxy authentication.

func (conf *config) readPasswordFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("could not open %s: %s", filename, err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Print(fmt.Errorf("could not close %s: %s", filename, err))
		}
	}(f)

	cr := newConfigReader(f)

	for {
		line, err := cr.ReadLine()
		if err != nil {
			break
		}

		words := strings.Fields(line)

		switch len(words) {
		case 2:
			conf.Passwords[words[0]] = words[1]

		case 3:
			user, pass, portStr := words[0], words[1], words[2]
			conf.Passwords[user] = pass
			port, err := strconv.Atoi(portStr)
			if err != nil {
				log.Printf("invalid port number %q in password file line: %s", portStr, line)
				continue
			}
			conf.CustomPorts[user] = customPortInfo{
				Port: port,
			}
			conf.UserForPort[port] = user

		case 4:
			user, pass, portStr, clientPlatform := words[0], words[1], words[2], words[3]
			conf.Passwords[user] = pass
			port, err := strconv.Atoi(portStr)
			if err != nil {
				log.Printf("invalid port number %q in password file line: %s", portStr, line)
				continue
			}
			conf.CustomPorts[user] = customPortInfo{
				Port:           port,
				ClientPlatform: clientPlatform,
			}
			conf.UserForPort[port] = user

		case 5:
			user, pass, portStr, clientPlatform, networks := words[0], words[1], words[2], words[3], words[4]
			conf.Passwords[user] = pass
			port, err := strconv.Atoi(portStr)
			if err != nil {
				log.Printf("invalid port number %q in password file line: %s", portStr, line)
				continue
			}
			conf.CustomPorts[user] = customPortInfo{
				Port:             port,
				ClientPlatform:   clientPlatform,
				ExpectedNetworks: strings.Split(networks, ","),
			}
			conf.UserForPort[port] = user

		default:
			log.Println("malformed line in password file:", line)
		}
	}

	return nil
}

func send407(w http.ResponseWriter) {
	c := getConfig()
	w.Header().Set("Proxy-Authenticate", "Basic realm="+c.AuthRealm)
	http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
}

// ProxyCredentials returns the username and password from r's
// Proxy-Authorization header.
func ProxyCredentials(r *http.Request) (user, pass string, ok bool) {
	auth := r.Header.Get("Proxy-Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Basic ") {
		return "", "", false
	}

	return decodeBase64Credentials(strings.TrimPrefix(auth, "Basic "))
}

// decodeBase64Credentials decodes a base64-encoded username:password pair
// such as those used in HTTP basic authentication.
func decodeBase64Credentials(auth string) (user, pass string, ok bool) {
	auth = strings.TrimSpace(auth)
	enc := base64.StdEncoding
	buf := make([]byte, enc.DecodedLen(len(auth)))
	n, err := enc.Decode(buf, []byte(auth))
	if err != nil {
		return "", "", false
	}
	auth = string(buf[:n])

	colon := strings.Index(auth, ":")
	if colon == -1 {
		return "", "", false
	}

	return auth[:colon], auth[colon+1:], true
}

// ValidCredentials returns whether user and password are a valid combination.
func (conf *config) ValidCredentials(user, password string) bool {
	conf.PasswordLock.RLock()
	ok := password == conf.Passwords[user] && password != ""
	conf.PasswordLock.RUnlock()
	if ok {
		return true
	}

	for _, a := range conf.Authenticators {
		if a(user, password) {
			if _, ok := conf.Passwords[user]; !ok {
				// Cache the password for later use.
				conf.PasswordLock.Lock()
				conf.Passwords[user] = password
				conf.PasswordLock.Unlock()
			}
			return true
		}
	}
	return false
}

func (conf *config) addAuthenticator(path string) error {
	if strings.ToLower(path) == "bypass" {
		conf.Authenticators = append(conf.Authenticators, func(user string, password string) bool {
			return true
		})
		return nil
	}
	conf.Authenticators = append(conf.Authenticators, func(user, password string) bool {
		cmd := exec.Command(path, user, password)
		err := cmd.Run()
		return err == nil
	})

	return nil
}

func (conf *config) addHTTPAuthenticator(endpoint string) error {
	var client http.Client
	client.Transport = transportWithExtraRootCerts

	conf.Authenticators = append(conf.Authenticators, func(user, password string) bool {
		formData := make(url.Values)
		formData.Set("username", user)
		formData.Set("password", password)
		resp, err := client.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
		if err != nil {
			log.Printf("Error communicating with authentication API endpoint %s: %v", endpoint, err)
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return false
		}

		var userInfo struct {
			DeviceGroups []string `json:"device_groups"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&userInfo); err == nil {
			conf.ACLs.ExternalDGLock.Lock()
			if conf.ACLs.ExternalDeviceGroups == nil {
				conf.ACLs.ExternalDeviceGroups = map[string][]string{}
			}
			conf.ACLs.ExternalDeviceGroups[user] = userInfo.DeviceGroups
			conf.ACLs.ExternalDGLock.Unlock()
		} else {
			log.Printf("Error decoding authenticator-api response for %s: %v", user, err)
		}

		return true
	})

	return nil
}

func (conf *config) loadIPToUser(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("could not open %s: %s", filename, err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) > 2 && strings.HasPrefix(fields[2], "#") {
			fields = fields[:2]
		}

		if len(fields) != 2 {
			log.Printf("Syntax error in %s: %q", filename, line)
			continue
		}

		conf.IPToUser[fields[0]] = fields[1]
	}

	return s.Err()
}
