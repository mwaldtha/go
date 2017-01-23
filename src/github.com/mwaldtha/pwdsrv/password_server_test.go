package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

//data for testing the actual hash/encoding process
var testPasswords = []struct {
	originalValue string
	expectedValue string
}{
	{"angryMonkey", "ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q=="},
	{"password", "sQnzu7wkTrgkQZF+0G1hi5AI3Qmzvv0bXgc5THBqi7mAsdd4Xll27ASbRt9fEyavWi6m0QP9B8lThf+rDKy8hg=="},
	{"two words", "rbeOtyY+z9LrVIi2SX/tw4jmeucGQ80oUx1v0gz+8FpP0Jw21auG5p4yJL721e+vj3/dcQIDt0C9+H3fKu8SVA=="},
	{"1234567890", "ErAyJqbYvpxujNXlXcbHkgyqo53xSquS1ePqk0DRyKTT0LjkMU8fbvExukvxzrkYarh8gBrw1clbG++4ztriuQ=="},
	{" !~@#d{}[]WQS67*/?", "1CPHe9u49v+FXpdubV0IYYvNtkUn38l02Ijbw7Jn8JkSu54TgeVNDm4mWTAzm8iCedAdiWrlyWZ2diPycf67+Q=="},
	{"Hello, 世界", "q5bnkSm2cCQbB/6S0TXdP5B6OKXUs2cn3J9HGgI+/jV+vIhKFuP1NvMYQ4nyB5gXf3IvisTGl+rHhfuZCHOLGw=="},
	{"", "A value must be submitted."},
	{"ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-ReallyLongPassword-", "KdRKHNcXx99I4FTpdQxqk4203FR1L8FwHDX0ovkuTB675g1c/BLMa78FSRguc6Ha/yIEF3+OxrFPnnSQqTML9A=="},
}

//data for testing how the HashHandler responds to various http methods
var hashHandlerHttpMethods = []struct {
	methodName   string
	expectedCode int
}{
	{http.MethodPost, http.StatusBadRequest},
	{http.MethodGet, http.StatusBadRequest},
	{http.MethodPut, http.StatusMethodNotAllowed},
	{http.MethodDelete, http.StatusMethodNotAllowed},
	{http.MethodHead, http.StatusMethodNotAllowed},
	{http.MethodPatch, http.StatusMethodNotAllowed},
}

//data for testing how the StatsHandler responds to various http methods
var statsHandlerHttpMethods = []struct {
	methodName   string
	expectedCode int
}{
	{http.MethodPost, http.StatusMethodNotAllowed},
	{http.MethodGet, http.StatusOK},
	{http.MethodPut, http.StatusMethodNotAllowed},
	{http.MethodDelete, http.StatusMethodNotAllowed},
	{http.MethodHead, http.StatusMethodNotAllowed},
	{http.MethodPatch, http.StatusMethodNotAllowed},
}

//Test to pass a simulated request through the HashHandler, hitting the POST processing code
func TestHashHandlerPost(t *testing.T) {
	handler := new(HashHandler)
	hashStats.Store(&HashStat{Total: 0, Average: 0})
	count := 0

	for _, testPassword := range testPasswords {
		recorder := httptest.NewRecorder()
		// create new request for each password in testPasswords
		// in this case the url specified below is not really used, so the value is irrelevant
		req, newReqErr := http.NewRequest(http.MethodPost, "http://localhost/", strings.NewReader(passwordFormName+"="+testPassword.originalValue))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")

		if newReqErr != nil {
			t.Error("An error occured while creating the request: ", newReqErr)
		}

		// process request through the handler
		handler.ServeHTTP(recorder, req)

		body := strings.TrimSpace(recorder.Body.String())

		if testPassword.originalValue != "" {
			count++
			jobInt, _ := strconv.Atoi(body)
			if jobInt != count {
				t.Errorf("HashHandlerPost test (%s): expected: 'Job ID: %d', actual: '%s'", testPassword.originalValue, count, body)
			}
		} else {
			if body != testPassword.expectedValue {
				t.Errorf("HashHandlerPost test (%s): expected: '%s', actual: '%s'", testPassword.originalValue, testPassword.expectedValue, body)
			}
		}
	}
}

//Test to pass a simulated request through the HashHandler, hitting the GET processing code
func TestHashHandlerGet(t *testing.T) {
	handler := new(HashHandler)
	count := int32(0)

	for _, testPassword := range testPasswords {
		recorder := httptest.NewRecorder()
		count++
		hashes[count] = testPassword.expectedValue

		// create new request for each password in testPasswords
		req, newReqErr := http.NewRequest(http.MethodGet, "http://localhost/hash/"+strconv.FormatInt(int64(count), 10), nil)

		if newReqErr != nil {
			t.Error("An error occured while creating the request: ", newReqErr)
		}

		// process request through the handler
		handler.ServeHTTP(recorder, req)

		body := strings.TrimSpace(recorder.Body.String())

		if body != testPassword.expectedValue {
			t.Errorf("HashHandlerGet test (%s): expected: '%s', actual '%s'", testPassword.originalValue, testPassword.expectedValue, body)
		}
	}
}

//Test to pass a simulated request through the StatsHandler, hitting the GET processing code
func TestStatsHandlerGet(t *testing.T) {
	handler := new(StatsHandler)

	for x := 1; x <= 10; x++ {
		average := float64(x) * 10.0
		hashStats.Store(&HashStat{Total: int32(x), Average: average})
		recorder := httptest.NewRecorder()

		// create new request for each password in testPasswords
		// in this case the url specified below is not really used, so the value is irrelevant
		req, newReqErr := http.NewRequest(http.MethodGet, "http://localhost/", nil)

		if newReqErr != nil {
			t.Error("An error occured while creating the request: ", newReqErr)
		}

		// process request through the handler
		handler.ServeHTTP(recorder, req)

		body := strings.TrimSpace(recorder.Body.String())
		bodyBytes := []byte(body)

		var jsonMap map[string]interface{}

		if err := json.Unmarshal(bodyBytes, &jsonMap); err != nil {
			t.Errorf("Unable to unmarshal the response body: '%s'", body)
		}

		total := jsonMap["total"].(float64)
		avg := jsonMap["average"].(float64)

		if total != float64(x) && avg != float64(average) {
			t.Errorf("StatsHandlerGet test - unexpected response: %s", body)
		}
	}
}

// Test how the HashHandler responds to various HTTP methods
// without starting up a server or passing in a password
func TestHashHandlerHTTPMethods(t *testing.T) {
	handler := new(HashHandler)

	for _, method := range hashHandlerHttpMethods {
		recorder := httptest.NewRecorder()
		// create new request for each HTTP method in httpMethods
		// in this case the url specified below is not really used, so the value is irrelevant
		req, newReqErr := http.NewRequest(method.methodName, "http://localhost/", nil)

		if newReqErr != nil {
			t.Error("An error occured while creating the request: ", newReqErr)
		}

		// process request through the handler
		handler.ServeHTTP(recorder, req)

		if recorder.Code != method.expectedCode {
			t.Errorf("HTTP %s request: expected: %d, actual: %d", method.methodName, method.expectedCode, recorder.Code)
		}
	}
}

// Test how the StatsHandler responds to various HTTP methods without starting up a server
func TestStatsHandlerHTTPMethods(t *testing.T) {
	handler := new(StatsHandler)
	hashStats.Store(&HashStat{Total: 0, Average: 0})

	for _, method := range statsHandlerHttpMethods {
		recorder := httptest.NewRecorder()
		// create new request for each HTTP method in httpMethods
		// in this case the url specified below is not really used, so the value is irrelevant
		req, newReqErr := http.NewRequest(method.methodName, "http://localhost/", nil)

		if newReqErr != nil {
			t.Error("An error occured while creating the request: ", newReqErr)
		}

		// process request through the handler
		handler.ServeHTTP(recorder, req)

		if recorder.Code != method.expectedCode {
			t.Errorf("HTTP %s request: expected: %d, actual: %d", method.methodName, method.expectedCode, recorder.Code)
		}
	}
}

//Test how the HashHandler responds to various HTTP methods
//through a running server without passing in a password
func TestHashHandlerServerHTTPMethods(t *testing.T) {
	testServer := httptest.NewServer(new(HashHandler))
	t.Logf("Running server at: %s", testServer.URL)

	// close the server after this test finishes
	defer testServer.Close()

	client := &http.Client{}

	for _, method := range hashHandlerHttpMethods {
		// create new request for each HTTP method in httpMethods
		req, newReqErr := http.NewRequest(method.methodName, testServer.URL, nil)

		if newReqErr != nil {
			t.Error("An error occured while creating the request: ", newReqErr)
		}

		// call client.Do() since HTTP method will differ
		actual, doErr := client.Do(req)

		if doErr != nil {
			t.Error("An error occured while submitting the request: ", doErr)
		}

		if actual.StatusCode != method.expectedCode {
			t.Errorf("HTTP %s request: expected: %d, actual: %d", method.methodName, method.expectedCode, actual.StatusCode)
		}
	}
}

//Test how the StatsHandler responds to various HTTP methods
//through a running server without passing in a password
func TestStatsHandlerServerHTTPMethods(t *testing.T) {
	testServer := httptest.NewServer(new(StatsHandler))
	t.Logf("Running server at: %s", testServer.URL)

	// close the server after this test finishes
	defer testServer.Close()

	client := &http.Client{}

	for _, method := range statsHandlerHttpMethods {
		// create new request for each HTTP method in httpMethods
		req, newReqErr := http.NewRequest(method.methodName, testServer.URL, nil)

		if newReqErr != nil {
			t.Error("An error occured while creating the request: ", newReqErr)
		}

		// call client.Do() since HTTP method will differ
		actual, doErr := client.Do(req)

		if doErr != nil {
			t.Error("An error occured while submitting the request: ", doErr)
		}

		if actual.StatusCode != method.expectedCode {
			t.Errorf("HTTP %s request: expected: %d, actual: %d", method.methodName, method.expectedCode, actual.StatusCode)
		}
	}
}

//Test how the HashHandler processes multiple parallel POST requests
func TestParallelHashHandlerPostRequests(t *testing.T) {

	// use a WaitGroup to wait for all goroutines in this test to finish
	var wg sync.WaitGroup
	hashStats.Store(&HashStat{Total: 0, Average: 0})

	testServer := httptest.NewServer(new(HashHandler))
	t.Logf("Running server at: %s", testServer.URL)

	// close the server after this test finishes
	defer testServer.Close()

	// loop through testPasswords 10 times
	for i := 0; i < 10; i++ {
		for _, testPassword := range testPasswords {
			wg.Add(1)
			t.Logf("%s - Submitting '%s'", time.Now().Format(time.StampNano), testPassword.originalValue)
			// run a separate goroutine for each password request
			go doHashHandlerPostRequest(testServer.URL, testPassword.originalValue, testPassword.expectedValue, t, &wg)
		}
	}
	wg.Wait()

	t.Log("Finshed parallel HashHandler POST test.")
}

// private function to issue POST requests to the HashHandler
// used by each goroutine in parallel testing
func doHashHandlerPostRequest(serverURL string, orig string, expected string, t *testing.T, wg *sync.WaitGroup) {

	//tell the WaitGroup when this processing is done
	defer wg.Done()

	client := &http.Client{}

	// create and assign password form value
	v := url.Values{}
	v.Set(passwordFormName, orig)

	// issue POST request with password form value
	postResponse, postErr := client.PostForm(serverURL, v)

	if postErr != nil {
		t.Error("An error occured while submitting the request: ", postErr)
	} else {
		defer postResponse.Body.Close()
		// read value returned in response body
		actual, readBodyErr := ioutil.ReadAll(postResponse.Body)

		if readBodyErr != nil {
			t.Error("Unable to read the response body: ", readBodyErr)
		} else {
			body := strings.TrimSpace(string(actual))

			if orig != "" {
				_, err := strconv.Atoi(body)
				if err != nil {
					t.Errorf("HashHandlerPost test (%s): Unable to convert job id '%s'", orig, body)
				}
			} else {
				if body != expected {
					t.Errorf("HashHandlerPost test (%s): expected: '%s', actual: '%s'", orig, expected, body)
				}
			}
		}
	}
}
