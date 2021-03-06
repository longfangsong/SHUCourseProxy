package handler

import (
	"SHUCourseProxy/infrastructure"
	"SHUCourseProxy/model"
	"SHUCourseProxy/service"
	"encoding/json"
	"github.com/dgrijalva/jwt-go"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
)

func getCookieJarFromRequest(r *http.Request, url string) (http.CookieJar, error) {
	tokenString := r.Header.Get("Authorization")[len("Bearer "):]
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		return nil, err
	}
	claims := token.Claims.(jwt.MapClaims)
	studentId := claims["studentId"].(string)
	siteId, err := model.GetSiteIdForURL(url)
	if err != nil {
		return nil, err
	}
	jar, err := model.GetCookieJar(studentId, siteId)
	if err != nil {
		return nil, err
	}
	return jar, nil
}

func postWithSaml(urlString string, samlRequest string, relayState string, client *http.Client) (*http.Response, error) {
	return client.PostForm(urlString, url.Values{
		"SAMLRequest": []string{samlRequest},
		"RelayState":  []string{relayState},
	})
}

func simulateLogin(fromURL string, studentId string, password string) http.CookieJar {
	jar, _ := cookiejar.New(nil)
	client := http.Client{
		Jar: jar,
	}
	_, err := client.Get(fromURL)
	infrastructure.CheckErr(err, "Cannot reach site"+fromURL)
	_, err = client.PostForm(`https://oauth.shu.edu.cn/login`, url.Values{
		"username":     []string{studentId},
		"password":     []string{password},
		"login_submit": []string{"登录"},
	})
	infrastructure.CheckErr(err, "failed to oauth")
	resp, err := client.Get(fromURL)
	infrastructure.CheckErr(err, "Target site "+fromURL+" still not available")
	content, err := ioutil.ReadAll(resp.Body)
	infrastructure.CheckErr(err, "Target site "+fromURL+" still not available")
	if strings.Contains(string(content), "id=\"login-submit\"") {
		return nil
	}
	return client.Jar
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	content, _ := ioutil.ReadAll(r.Body)
	var input struct {
		FromUrl  string `json:"from_url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	err := json.Unmarshal(content, &input)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	jar := simulateLogin(input.FromUrl, input.Username, input.Password)
	if jar == nil {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		siteId, err := model.GetOrCreateSiteIdForURL(input.FromUrl)
		infrastructure.CheckErr(err, "GetOrCreateSiteIdForURL failed")
		model.SetCookieJar(input.Username, siteId, jar)
		_, _ = w.Write([]byte(service.GenerateJWT(input.Username)))
	}
}

func GetWithCookieHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	var input struct {
		Url string `json:"url"`
	}
	err = json.Unmarshal(body, &input)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	jar, err := getCookieJarFromRequest(r, input.Url)
	if err != nil {
		w.WriteHeader(403)
		return
	}
	result, _ := service.GetWithCookieJar(input.Url, jar)
	_, err = w.Write(result)
	if err != nil {
		w.WriteHeader(500)
	}
}

func PostWithCookieHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	type Content struct {
		Url     string      `json:"url"`
		Content interface{} `json:"content"`
	}
	var content Content
	err = json.Unmarshal(body, &content)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	jar, err := getCookieJarFromRequest(r, content.Url)
	if err != nil {
		w.WriteHeader(403)
		return
	}
	encoded, err := json.Marshal(content.Content)
	result, _ := service.PostJsonWithCookieJar(content.Url, encoded, jar)
	_, err = w.Write(result)
	if err != nil {
		w.WriteHeader(500)
	}
}

func PostFormWithCookieHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	type Content struct {
		Url     string            `json:"url"`
		Content map[string]string `json:"content"`
	}
	var content Content
	err = json.Unmarshal(body, &content)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	jar, err := getCookieJarFromRequest(r, content.Url)
	if err != nil {
		w.WriteHeader(403)
		return
	}
	result, _ := service.PostFormWithCookieJar(content.Url, content.Content, jar)
	_, err = w.Write(result)
	if err != nil {
		w.WriteHeader(500)
	}
}
