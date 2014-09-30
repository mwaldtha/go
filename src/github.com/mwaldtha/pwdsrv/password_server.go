package main 

import (
    "crypto/sha512"
    "encoding/base64"
    "flag"
    "fmt"
    "log"
    "net/http"
    "time"
)

const (
    postMethod string = "POST"
    passwordFormName string = "password"
)

type PasswordHandler struct{}

func main() {
    // process command line flag for port number or default to 8080
    portFlag := flag.Int("port", 8080, "Port number the server will listen on.")
    
    flag.Parse()
    
    log.Printf("Starting server on port %d", *portFlag)
    err := http.ListenAndServe(fmt.Sprintf(":%d", *portFlag), new(PasswordHandler))
    
    if err != nil {
        log.Fatal("An error was received during ListenAndServe: ", err)
    }
}

func (ph PasswordHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // only support POST requests
    // return http.StatusMethodNotAllowed if any other HTTP method
    switch r.Method {
        case postMethod: 
        	p := r.PostFormValue(passwordFormName)
        	if p != "" {
        	    // wait 5 seconds before responding
        		time.Sleep(time.Duration(5)*time.Second)
        		w.Write([]byte(encryptAndEncode(r.PostFormValue(passwordFormName))))
        	} else {
        	    // return http.StatusBadRequest if no value was submitted
        	    w.WriteHeader(http.StatusBadRequest)
        	    w.Write([]byte("A value must be submitted."))
        	}
        default: http.Error(w, "Only POST requests are supported.", http.StatusMethodNotAllowed)
    }
}

// private function to encrypt and encode
// the supplied value
func encryptAndEncode(data string) string {
    log.Printf("Processing: '%s'\n", data)
    
    h := sha512.New()
    h.Write([]byte(data))
    cs := h.Sum(nil);
    
    return base64.StdEncoding.EncodeToString(cs)
}