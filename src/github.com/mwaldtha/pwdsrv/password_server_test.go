package main

import (
    "io/ioutil"
    "net/http"
    "net/http/httptest"
    "net/url"
    "testing"
    "time"
    "strings"
    "sync"
)

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

var httpMethods = []struct {
    methodName string
    expectedCode int
}{
    {"POST", http.StatusBadRequest},
    {"GET", http.StatusMethodNotAllowed},
    {"PUT", http.StatusMethodNotAllowed},
    {"DELETE", http.StatusMethodNotAllowed},
    {"HEAD", http.StatusMethodNotAllowed},
}

// Test various passwords serially through the handler
// not through a running server
func TestPasswords(t *testing.T) {
    handler := new(PasswordHandler)
    
    for _, testPassword := range testPasswords {
	    recorder := httptest.NewRecorder()
	    // create new request for each password in testPasswords
	    // in this case the url specified below is not really used, so the value is irrelevant
	    req, newReqErr := http.NewRequest("POST", "http://localhost/", strings.NewReader(passwordFormName + "=" + testPassword.originalValue))
	    req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	    
	    if newReqErr != nil {
	        t.Error("An error occured while creating the request: ", newReqErr)
	    }
	    
	    // process request through the handler
	    handler.ServeHTTP(recorder, req)
	    
	    if recorder.Body.String() != testPassword.expectedValue {
	        t.Errorf("Password test (%s): expected: %s, actual: %s", testPassword.originalValue, testPassword.expectedValue, recorder.Body.String())
	    }
	}
}

// Test how the handler responds to various HTTP methods
// without starting up a server or passing in a password
func TestHandlerHTTPMethods(t *testing.T) {
    handler := new(PasswordHandler)
    
    for _, method := range httpMethods {
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

// Test how the handler responds to various HTTP methods
// through a running server without passing in a password
func TestServerHTTPMethods(t *testing.T) {
    testServer := httptest.NewServer(new(PasswordHandler))
    t.Logf("Running server at: %s", testServer.URL)
    
    // close the server after this test finishes
    defer testServer.Close()
    
    client := &http.Client{}
    
	for _, method := range httpMethods {
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

// Test submitting passwords in parallel
// through a running server
func TestParallelRequests(t *testing.T) {
    
    // use a WaitGroup to wait for all goroutines in this test to finish
    var wg sync.WaitGroup
    
    testServer := httptest.NewServer(new(PasswordHandler))
    t.Logf("Running server at: %s", testServer.URL)
    
    // close the server after this test finishes
    defer testServer.Close()

	// loop through testPasswords 10 times
	for i := 0; i < 10; i++ {
	    for _, testPassword := range testPasswords {
	        wg.Add(1)
	        t.Logf("%s - Submitting '%s'", time.Now().Format(time.StampNano), testPassword.originalValue)
	        // run a seperate goroutine for each password request
	        go doPostRequest(testServer.URL, testPassword.originalValue, testPassword.expectedValue, t, &wg)
	   	}
    }
	wg.Wait()
    
    t.Log("Finshed parallel tests.")
}

// private function to issue POST requests
// used by each goroutine in parallel testing
func doPostRequest(serverURL string, orig string, expected string, t *testing.T, wg *sync.WaitGroup) {
    
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
   		    if string(actual) != expected {
   		        t.Errorf("Password test (%s): expected: %s, actual: %s", orig, expected, string(actual))
		    }
   		}
   	}
}



