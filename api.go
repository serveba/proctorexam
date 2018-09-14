package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// https://medium.com/@marcus.olsson/writing-a-go-client-for-your-restful-api-c193a2f4998c
// http://docs.proctorexam.com/v3/apidoc.html

// Exam data struct
type Exam struct {
	foo string
}

// Client sdk structure
type Client struct {
	BaseURL      *url.URL
	httpClient   *http.Client
	UserAgent    string
	apiKey       string
	apiSecretKey string
}

func (c *Client) newRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	ts := strconv.FormatUint(uint64(time.Now().UnixNano()/int64(time.Millisecond)), 10)
	nonce := strconv.FormatUint(uint64(random(0, 10000000000000000)), 10)

	params := map[string]string{
		"nonce":     nonce,
		"timestamp": ts,
	}

	signature := c.signParams(params)
	target := fmt.Sprintf("%s?nonce=%s&timestamp=%s&signature=%s", u.String(), nonce, ts, signature)

	// fmt.Printf("target: %s", target)
	req, err := http.NewRequest(method, target, buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/vnd.procwise.v3")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Authorization", "Token token="+c.apiKey)

	return req, nil
}

func (c *Client) do(req *http.Request, v interface{}) (*http.Response, error) {
	formatRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	bodyString := string(bodyBytes)
	fmt.Printf("RESPONSE: \n%s\n", bodyString)

	err = json.NewDecoder(resp.Body).Decode(v)
	return resp, err
}

// same function as:
// https://gist.github.com/almeidabbm/c1e1f184572674f7c7cea193d0b55ea7
func (c *Client) signParams(params map[string]string) string {
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	baseString := ""
	for i := range keys {
		baseString = fmt.Sprintf("%s?%s=%s", baseString, keys[i], params[keys[i]])
	}

	hash := hmac.New(sha256.New, []byte(c.apiSecretKey))
	hash.Write([]byte(baseString))
	signature := hex.EncodeToString(hash.Sum(nil))

	fmt.Printf("baseString: %s\n", baseString)
	fmt.Printf("Signature: %s\n", signature)

	return signature
}

func random(min, max int64) int64 {
	rand.Seed(time.Now().Unix())
	return rand.Int63n(max-min) + min
}

// ListExams method
func (c *Client) ListExams() ([]Exam, error) {
	req, err := c.newRequest("GET", "/api/v3/exams", nil)
	if err != nil {
		return nil, err
	}
	var exams []Exam
	_, err = c.do(req, &exams)
	return exams, err
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v\n", r.Proto, r.Method, r.URL)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v\n", r.Host))
	// Loop through headers
	request = append(request, fmt.Sprintf("Headers: \n"))
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("\t%v: %v\n", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	// Return the request as a string

	fmt.Printf("REQUEST: \n%s\n", request)

	return strings.Join(request, "\n")
}

func main() {
	baseURL, err := url.Parse(os.Getenv("PE_ENDPOINT"))
	if err != nil {
		panic(err)
	}

	c := &Client{
		httpClient:   http.DefaultClient,
		UserAgent:    "ProctorExam golang SDK user agent",
		BaseURL:      baseURL,
		apiKey:       os.Getenv("PE_API_KEY"),
		apiSecretKey: os.Getenv("PE_API_SECRET_KEY"),
	}

	fmt.Printf("domain: %s, key: %s, secret: %s\n", os.Getenv("PE_ENDPOINT"),
		os.Getenv("PE_API_KEY"), os.Getenv("PE_SECRET_KEY"))

	exams, err := c.ListExams()
	if err != nil {
		panic(err)
	}

	fmt.Printf("exams: %s\n", exams)
}
