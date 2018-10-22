package proctorexam

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
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// http://docs.proctorexam.com/v3/apidoapi.html

const apiPrefix string = "/api/v3"

// Exam data struct
type Exam struct {
	ID          int64  `json:"id"`
	InstituteID int64  `json:"institute_id"`
	Name        string `json:"name"`
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

// Student nested object of GET /exams/id/show_student
type Student struct {
	ID     int64  `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
	ExamID int64  `json:"exam_id"`
}

// API ProctorExam sdk metadata
type API struct {
	baseURL      *url.URL
	httpClient   *http.Client
	userAgent    string
	debug        bool
	apiKey       string
	apiSecretKey string
}

// Option is a functional option for configuring the API client
type Option func(*API) error

// parseOptions parses the supplied options functions and returns a configured
// *Client instance
func (api *API) parseOptions(opts ...Option) error {
	// Range over each options function and apply it to our API type to
	// configure it. Options functions are applied in order, with any
	// conflicting options overriding earlier calls.
	for _, option := range opts {
		err := option(api)
		if err != nil {
			return err
		}
	}

	return nil
}

// BaseURL allows overriding of API client baseURL for testing
func BaseURL(baseURL *url.URL) Option {
	return func(api *API) error {
		api.baseURL = baseURL
		return nil
	}
}

// New creates a new API client
func New(opts ...Option) (*API, error) {
	// url, _ := url.Parse(apiURL)
	client := &API{
		// baseURL: url,
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
		debug: false,
	}

	if err := client.parseOptions(opts...); err != nil {
		return nil, err
	}

	return client, nil
}

func (api *API) newGetRequest(path string, params, queryParams map[string]string) (*http.Request, error) {
	return api.newRequest("GET", path, nil, params, queryParams)
}

// same function as:
// https://gist.github.com/almeidabbm/c1e1f184572674f7c7cea193d0b55ea7
func (api *API) signParams(params map[string]string) string {
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

	hash := hmac.New(sha256.New, []byte(api.apiSecretKey))
	hash.Write([]byte(baseString))
	signature := hex.EncodeToString(hash.Sum(nil))

	// fmt.Printf("baseString: %s\n", baseString)
	// fmt.Printf("Signature: %s\n", signature)

	return signature
}

func (api *API) newRequest(method, path string, body interface{}, params, queryParams map[string]string) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := api.baseURL.ResolveReference(rel)
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	signature := api.signParams(params)

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
	req.Header.Set("User-Agent", api.userAgent)
	req.Header.Set("Authorization", "Token token="+api.apiKey)

	return req, nil
}

func (api *API) do(req *http.Request, v interface{}) error {
	if api.debug {
		reqDump, err := httputil.DumpRequest(req, true)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s", reqDump)
	}

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if api.debug {
		bodyString := string(bodyBytes)
		fmt.Printf("RESPONSE: \n%s\n", bodyString)
	}

	err = json.Unmarshal(bodyBytes, v)
	// err = json.NewDecoder(resp.Body).Decode(v)
	return err
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
func (api *API) Exams() ([]Exam, error) {
	path := fmt.Sprintf("%s/exams", apiPrefix)
	params := getBaseParams()
	req, err := api.newGetRequest(path, params, nil)
	if err != nil {
		return nil, err
	}
	type examsWrapper struct {
		Items []Exam `json:"exams"`
	}
	var exams examsWrapper
	err = api.do(req, &exams)

	return exams.Items, err
}

// Exam GET /exams/:id
func (api *API) Exam(id int64) (Exam, error) {
	path := fmt.Sprintf("%s/exams/%d", apiPrefix, id)
	params := getBaseParams()
	params["id"] = strconv.Itoa(int(id))
	req, err := api.newGetRequest(path, params, nil)
	if err != nil {
		return Exam{}, err
	}
	type examWrapper struct {
		Key Exam `json:"exam"`
	}
	var exam examWrapper
	err = api.do(req, &exam)

	return exam.Key, err
}

// Users GET /institutes/:institute_id/users
func (api *API) Users(instituteID int64) ([]User, error) {
	path := fmt.Sprintf("%s/institutes/%d/users", apiPrefix, instituteID)
	params := getBaseParams()
	params["institute_id"] = strconv.Itoa(int(instituteID))
	req, err := api.newGetRequest(path, params, nil)
	if err != nil {
		return nil, err
	}
	type usersWrapper struct {
		Items []User `json:"users"`
	}
	var users usersWrapper
	err = api.do(req, &users)

	return users.Items, err
}

// ShowUser GET /institutes/:institute_id/users/:id
func (api *API) ShowUser(instituteID, userID int64) (User, error) {
	path := fmt.Sprintf("%s/institutes/%d/users/%d", apiPrefix, instituteID, userID)
	params := getBaseParams()
	params["id"] = strconv.Itoa(int(userID))
	params["institute_id"] = strconv.Itoa(int(instituteID))
	req, err := api.newGetRequest(path, params, nil)
	if err != nil {
		return User{}, err
	}
	type userWrapper struct {
		Item User `json:"user"`
	}
	var user userWrapper
	err = api.do(req, &user)

	return user.Item, err
}

// ShowStudent GET /exams/:id/show_student?student_session_id=
func (api *API) ShowStudent(examID, studentSessionID int64) (Student, error) {
	path := fmt.Sprintf("%s/exams/%d/show_student", apiPrefix, examID)
	params := getBaseParams()
	sessionID := strconv.Itoa(int(studentSessionID))
	params["student_session_id"] = sessionID
	params["id"] = strconv.Itoa(int(examID))
	req, err := api.newGetRequest(path, params, map[string]string{"student_session_id": sessionID})
	if err != nil {
		return Student{}, err
	}
	type studentWrapper struct {
		Item Student `json:"student"`
	}
	var wrapper studentWrapper
	err = api.do(req, &wrapper)

	return wrapper.Item, err
}

// IndexStudents GET /exams/:id/index_students
func (api *API) IndexStudents(examID int64) ([]Student, error) {
	path := fmt.Sprintf("%s/exams/%d/index_students", apiPrefix, examID)
	params := getBaseParams()
	params["id"] = strconv.Itoa(int(examID))
	req, err := api.newGetRequest(path, params, nil)
	if err != nil {
		return nil, err
	}
	type studentsWrapper struct {
		Items []Student `json:"students"`
	}
	var students studentsWrapper
	err = api.do(req, &students)

	return students.Items, nil
}
