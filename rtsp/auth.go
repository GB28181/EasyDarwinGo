package rtsp

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// BasicAuth of RTSP
func BasicAuth(authLine string, method string, URL string) (string, error) {
	l, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("Url parse error:%v,%v", URL, err)
	}

	username := l.User.Username()
	password, _ := l.User.Password()
	basic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))

	authorization := fmt.Sprintf("Basic %s", basic)

	return authorization, nil
}

// DigestAuth of RTSP
func DigestAuth(authLine string, method string, URL string) (string, error) {
	l, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("Url parse error:%v,%v", URL, err)
	}
	realm := ""
	nonce := ""
	realmRex := regexp.MustCompile(`realm="(.*?)"`)
	result1 := realmRex.FindStringSubmatch(authLine)

	nonceRex := regexp.MustCompile(`nonce="(.*?)"`)
	result2 := nonceRex.FindStringSubmatch(authLine)

	if len(result1) == 2 {
		realm = result1[1]
	} else {
		return "", fmt.Errorf("auth error : no realm found")
	}
	if len(result2) == 2 {
		nonce = result2[1]
	} else {
		return "", fmt.Errorf("auth error : no nonce found")
	}
	// response= md5(md5(username:realm:password):nonce:md5(public_method:url));
	username := l.User.Username()
	password, _ := l.User.Password()
	l.User = nil
	if l.Port() == "" {
		l.Host = fmt.Sprintf("%s:%s", l.Host, "554")
	}
	md5UserRealmPwd := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
	md5MethodURL := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s", method, l.String()))))

	response := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", md5UserRealmPwd, nonce, md5MethodURL))))
	Authorization := fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", response=\"%s\"", username, realm, nonce, l.String(), response)
	return Authorization, nil
}

func (client *RTSPClient) checkAuth(method string, resp *Response) (string, error) {
	if resp.StatusCode == 401 {

		// need auth.
		AuthHeaders := resp.Header["WWW-Authenticate"]

		if auths, ok := AuthHeaders.([]string); ok {
			for _, authLine := range auths {

				if strings.IndexAny(authLine, "Digest") == 0 {
					// 					realm="HipcamRealServer",
					// nonce="3b27a446bfa49b0c48c3edb83139543d"
					client.authLine = authLine
					return DigestAuth(authLine, method, client.URL)
				} else if strings.IndexAny(authLine, "Basic") == 0 {
					client.authLine = authLine
					return BasicAuth(authLine, method, client.URL)
				}

			}
			return "", fmt.Errorf("auth method nnot support, %v", auths)
		} else if authLine, ok := AuthHeaders.(string); ok {
			if strings.IndexAny(authLine, "Digest") == 0 {
				client.authLine = authLine
				return DigestAuth(authLine, method, client.URL)
			} else if strings.IndexAny(authLine, "Basic") == 0 {
				client.authLine = authLine
				return BasicAuth(authLine, method, client.URL)
			}
		}
	}
	return "", nil
}

func CheckAuth(authLine string, method string, sessionNonce string) error {
	realmRex := regexp.MustCompile(`realm="(.*?)"`)
	nonceRex := regexp.MustCompile(`nonce="(.*?)"`)
	usernameRex := regexp.MustCompile(`username="(.*?)"`)
	responseRex := regexp.MustCompile(`response="(.*?)"`)
	uriRex := regexp.MustCompile(`uri="(.*?)"`)

	realm := ""
	nonce := ""
	username := ""
	response := ""
	uri := ""
	result1 := realmRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		realm = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : no realm found")
	}
	result1 = nonceRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		nonce = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : no nonce found")
	}
	if sessionNonce != nonce {
		return fmt.Errorf("CheckAuth error : sessionNonce not same as nonce")
	}

	result1 = usernameRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		username = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : username not found")
	}

	result1 = responseRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		response = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : response not found")
	}

	result1 = uriRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		uri = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : uri not found")
	}

	// TODO: query user

	md5UserRealmPwd := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, "user.Password"))))
	md5MethodURL := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s", method, uri))))
	myResponse := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", md5UserRealmPwd, nonce, md5MethodURL))))
	if myResponse != response {
		return fmt.Errorf("CheckAuth error : response not equal")
	}
	return fmt.Errorf("CheckAuth error : user not exists")
}
