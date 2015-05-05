package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIHandlerWrongUrl(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost:8801/strange/url", nil)
	if err != nil {
		log.Fatal(err)
	}
	w := httptest.NewRecorder()
	apiHandler(w, req)
	expectedCode := 404
	if expectedCode != w.Code {
		t.Errorf("%v HTTP code not equal to expected %v", w.Code, expectedCode)
	}
}

func TestAPIHandlerWrongContainerName(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost:8801/api/v1.0/docker/wrongname/processes", nil)
	if err != nil {
		log.Fatal(err)
	}
	w := httptest.NewRecorder()
	apiHandler(w, req)
	expectedCode := 500
	if expectedCode != w.Code {
		t.Errorf("%v HTTP code not equal to expected %v", w.Code, expectedCode)
	}
}

func TestAPIHandlerWrongConstraints(t *testing.T) {
	containerName := "/docker/020361380fe7e4abf7117201cd2936a9cbbac757a78365651c38294eabe43ef0"
	urls := [...]string{
		fmt.Sprintf("http://localhost:8801/api/v1.0%s/processes?interval=100", containerName),
		fmt.Sprintf("http://localhost:8801/api/v1.0%s/processes?count=100", containerName),
	}
	for _, u := range urls {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			log.Fatal(err)
		}
		w := httptest.NewRecorder()
		apiHandler(w, req)
		expectedCode := 500
		if expectedCode != w.Code {
			t.Errorf("%v HTTP code not equal to expected %v", w.Code, expectedCode)
		}
	}
}
func TestAPIHandlerNormal(t *testing.T) {
	containerName := "/docker/020361380fe7e4abf7117201cd2936a9cbbac757a78365651c38294eabe43ef0"
	urls := [...]string{
		fmt.Sprintf("http://localhost:8801/api/v1.0%s/processes", containerName),
		fmt.Sprintf("http://localhost:8801/api/v1.0%s/processes?sort=mem", containerName),
		fmt.Sprintf("http://localhost:8801/api/v1.0%s/processes?sort=cpu", containerName),
	}
	collectData("process/testroot/")
	collectData("process/testroot/")
	for _, u := range urls {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			log.Fatal(err)
		}
		w := httptest.NewRecorder()
		apiHandler(w, req)
		expectedCode := 200
		if expectedCode != w.Code {
			t.Errorf("%v HTTP code not equal to expected %v for url %v", w.Code, expectedCode, u)
		}
	}
}
