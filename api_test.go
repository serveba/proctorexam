package proctorexam

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	mux    *http.ServeMux
	server *httptest.Server
	api    *API
)

const (
	idInst        = 17
	idUser        = 11
	idExam        = 17
	idStudent     = 804
	idStudSession = 4
)

func setup() func() {
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)
	url, _ := url.Parse(server.URL)
	api, _ = New(BaseURL(url))
	// for debugging requests and library
	// api.debug = true
	return func() {
		server.Close()
	}
}

func fixture(path string) string {
	b, err := ioutil.ReadFile("testdata/fixtures/" + path)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// https://www.markphelps.me/testing-api-clients-in-go/
func TestExams(t *testing.T) {
	teardown := setup()
	defer teardown()

	mux.HandleFunc("/api/v3/exams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, fixture("exams.json"))
	})

	exams, err := api.Exams()
	if err != nil {
		t.Fatal(err)
	}

	for _, exam := range exams {
		if exam.ID <= 0 && exam.InstituteID <= 0 && exam.Name != "" {
			t.Fatalf("Error with exam: %v", exam)
		}
	}
	assert.Equal(t, len(exams), 23)

}

func TestExam(t *testing.T) {
	teardown := setup()
	defer teardown()

	path := fmt.Sprintf("/api/v3/exams/%d", idExam)

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, fixture("exam.json"))
	})

	exam, err := api.Exam(idExam)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, int(exam.ID), idExam)

}

func TestUsers(t *testing.T) {
	teardown := setup()
	defer teardown()

	path := fmt.Sprintf("/api/v3/institutes/%d/users", idInst)

	mux.HandleFunc(path,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, fixture("users.json"))
		})

	users, err := api.Users(idInst)
	if err != nil {
		t.Fatal(err)
	}

	for _, user := range users {

		if user.ID <= 0 {
			t.Fatalf("Error with user: %v", user)
		}
	}
	assert.Equal(t, len(users), 4)
}

func TestShowUser(t *testing.T) {
	teardown := setup()
	defer teardown()

	path := fmt.Sprintf("/api/v3/institutes/%d/users/%d", idInst, idUser)

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, fixture("user.json"))
	})

	user, err := api.ShowUser(idInst, idUser)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, int(user.ID), idUser)
}

func TestShowStudent(t *testing.T) {
	teardown := setup()
	defer teardown()

	path := fmt.Sprintf("/api/v3/exams/%d/show_student", idExam)

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, fixture("student.json"))
	})

	student, err := api.ShowStudent(idExam, idStudSession)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, int(student.ID), idStudent)
}

func TestIndexStudents(t *testing.T) {
	teardown := setup()
	defer teardown()

	path := fmt.Sprintf("/api/v3/exams/%d/index_students", idInst)

	mux.HandleFunc(path,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, fixture("index_students.json"))
		})

	students, err := api.IndexStudents(idExam)
	if err != nil {
		t.Fatal(err)
	}

	for _, student := range students {

		if student.ID <= 0 {
			t.Fatalf("Error with student: %v", student)
		}
	}
	assert.Equal(t, len(students), 1)
	assert.Equal(t, int(students[0].ID), idStudent)
}
