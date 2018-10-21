package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"
)

// https://medium.com/@marcus.olsson/writing-a-go-client-for-your-restful-api-c193a2f4998c
// http://docs.proctorexam.com/v3/apidoc.html

const apiPrefix string = "/api/v3"

// Exams data struct
type Exams struct {
	Items []ExamItem `json:"exams"`
}

// ExamItem data struct
type ExamItem struct {
	ID          int64  `json:"id"`
	InstituteID int64  `json:"institute_id"`
	Name        string `json:"name"`
}

// Exam data struct
type Exam struct {
	Key ExamItem `json:"exam"`
}

// User internal data of user response
type User struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Role          string `json:"role"`
	LogoImg       string `json:"logo_image"`
	InstituteName string `json:"institute_name"`
}

// UserWrapper json wrapper for get user response
type UserWrapper struct {
	Item User `json:"user"`
}

// Users response of GET /USERS
type Users struct {
	Items []User `json:"users"`
}

// Student nested object of GET /exams/id/show_student
type Student struct {
	ID     int64  `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
	ExamID int64  `json:"exam_id"`
}

// StudentWrapper high level response of /exams/id/show_student
type StudentWrapper struct {
	Item Student `json:"student"`
}

// Students reponse of GET /exams/:id/index_students
type Students struct {
	Items []Student `json:"students"`
}

// Client ProctorExam sdk metadata
type Client struct {
	BaseURL      *url.URL
	httpClient   *http.Client
	UserAgent    string
	apiKey       string
	apiSecretKey string
}

func (c *Client) newGetRequest(path string, params, queryParams map[string]string) (*http.Request, error) {
	return c.newRequest("GET", path, nil, params, queryParams)
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
		if len(baseString) == 0 {
			baseString = fmt.Sprintf("%s=%s", keys[i], params[keys[i]])
		} else {
			baseString = fmt.Sprintf("%s?%s=%s", baseString, keys[i], params[keys[i]])
		}
	}

	hash := hmac.New(sha256.New, []byte(c.apiSecretKey))
	hash.Write([]byte(baseString))
	signature := hex.EncodeToString(hash.Sum(nil))

	// fmt.Printf("baseString: %s\n", baseString)
	// fmt.Printf("Signature: %s\n", signature)

	return signature
}

func (c *Client) newRequest(method, path string, body interface{}, params, queryParams map[string]string) (*http.Request, error) {
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
	signature := c.signParams(params)

	target := fmt.Sprintf("%s?nonce=%s&timestamp=%s&signature=%s",
		u.String(), params["nonce"], params["timestamp"], signature)

	if len(queryParams) > 0 {
		for key, value := range queryParams {
			target = fmt.Sprintf("%s&%s=%s", target, key, value)
		}

	}

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
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", reqDump)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// ONLY for RESPONSE DEBUG
	// bodyBytes, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return nil, err
	// }
	// bodyString := string(bodyBytes)
	// fmt.Printf("RESPONSE: \n%s\n", bodyString)
	// END DEBUGGING

	err = json.NewDecoder(resp.Body).Decode(v)
	return resp, err
}

func random(min, max int64) int64 {
	rand.Seed(time.Now().Unix())
	return rand.Int63n(max-min) + min
}

func getBaseParams() map[string]string {
	ts := strconv.FormatUint(uint64(time.Now().UnixNano()/int64(time.Millisecond)), 10)
	nonce := strconv.FormatUint(uint64(random(0, 10000000000000000)), 10)
	return map[string]string{
		"nonce":     nonce,
		"timestamp": ts,
	}
}

// Exams method
func (c *Client) Exams() (Exams, error) {
	path := fmt.Sprintf("%s/exams", apiPrefix)
	params := getBaseParams()
	req, err := c.newGetRequest(path, params, nil)
	if err != nil {
		return Exams{}, err
	}
	var exams Exams
	_, err = c.do(req, &exams)
	return exams, err
}

// Exam GET /exams/:id
func (c *Client) Exam(id int64) (ExamItem, error) {
	path := fmt.Sprintf("%s/exams/%d", apiPrefix, id)
	params := getBaseParams()
	params["id"] = strconv.Itoa(int(id))
	req, err := c.newGetRequest(path, params, nil)
	if err != nil {
		return ExamItem{}, err
	}
	var exam Exam
	_, err = c.do(req, &exam)

	return exam.Key, err
}

// Users GET /institutes/:institute_id/users
func (c *Client) Users(instituteID int64) ([]User, error) {
	path := fmt.Sprintf("%s/institutes/%d/users", apiPrefix, instituteID)
	params := getBaseParams()
	params["institute_id"] = strconv.Itoa(int(instituteID))
	req, err := c.newGetRequest(path, params, nil)
	if err != nil {
		return nil, err
	}
	var users Users
	_, err = c.do(req, &users)

	return users.Items, err
}

// ShowUser GET /institutes/:institute_id/users/:id
func (c *Client) ShowUser(instituteID, userID int64) (User, error) {
	path := fmt.Sprintf("%s/institutes/%d/users/%d", apiPrefix, instituteID, userID)
	params := getBaseParams()
	// params["id"] = strconv.Itoa(int(userID))
	// params["institute_id"] = strconv.Itoa(int(instituteID))
	req, err := c.newGetRequest(path, params, nil)
	if err != nil {
		return User{}, err
	}
	var user UserWrapper
	_, err = c.do(req, &user)

	return user.Item, err
}

// ShowStudent GET /exams/:id/show_student?student_session_id=
func (c *Client) ShowStudent(examID, studentSessionID int64) (Student, error) {
	path := fmt.Sprintf("%s/exams/%d/show_student", apiPrefix, examID)
	params := getBaseParams()
	sessionID := strconv.Itoa(int(studentSessionID))
	params["student_session_id"] = sessionID
	params["id"] = strconv.Itoa(int(examID))
	req, err := c.newGetRequest(path, params, map[string]string{"student_session_id": sessionID})
	if err != nil {
		return Student{}, err
	}
	var studentWrapper StudentWrapper
	_, err = c.do(req, &studentWrapper)

	return studentWrapper.Item, err
}

// IndexStudents GET /exams/:id/index_students
func (c *Client) IndexStudents(examID int64) ([]Student, error) {
	path := fmt.Sprintf("%s/exams/%d/index_students", apiPrefix, examID)
	params := getBaseParams()
	params["id"] = strconv.Itoa(int(examID))

	fmt.Printf("%v\n", params)
	req, err := c.newGetRequest(path, params, nil)

	if err != nil {
		return nil, err
	}
	var students Students
	_, err = c.do(req, &students)

	fmt.Printf("Users: %v\n", students)
	return students.Items, nil
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

	// exams, err := c.Exams()
	// fmt.Printf("exams: %v\n", *exams)

	// users, err := c.Users(1)
	// fmt.Printf("users: %v\n", users)

	// user, err := c.ShowUser(1, 11)
	// fmt.Printf("user: %v\n", user)

	// exam, err := c.Exam(70)
	// fmt.Printf("exam: %v\n", exam)

	// students, err := c.IndexStudents(70)
	// fmt.Printf("students: %v\n", students)

	student, err := c.ShowStudent(70, 804)
	fmt.Printf("student: %v\n", student)
}
