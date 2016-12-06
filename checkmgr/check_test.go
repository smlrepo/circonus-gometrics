// Copyright 2016 Circonus, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package checkmgr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/circonus-labs/circonus-gometrics/api"
)

var (
	testCheck = api.Check{
		CID:            "/check/1234",
		Active:         true,
		BrokerCID:      "/broker/1234",
		CheckBundleCID: "/check_bundle/1234",
		CheckUUID:      "abc123-a1b2-c3d4-e5f6-123abc",
		Details: api.CheckDetails{
			SubmissionURL: "http://127.0.0.1:43191/module/httptrap/abc123-a1b2-c3d4-e5f6-123abc/blah",
		},
	}

	testCheckBundle = api.CheckBundle{
		CheckUUIDs:    []string{"abc123-a1b2-c3d4-e5f6-123abc"},
		Checks:        []string{"/check/1234"},
		CID:           "/check_bundle/1234",
		Created:       0,
		LastModified:  0,
		LastModifedBy: "",
		ReverseConnectURLs: []string{
			"mtev_reverse://127.0.0.1:43191/check/abc123-a1b2-c3d4-e5f6-123abc",
		},
		Brokers:     []string{"/broker/1234"},
		DisplayName: "test check",
		Config: api.CheckBundleConfig{
			SubmissionURL: "http://127.0.0.1:43191/module/httptrap/abc123-a1b2-c3d4-e5f6-123abc/blah",
			ReverseSecret: "blah",
		},
		Metrics: []api.CheckBundleMetric{
			api.CheckBundleMetric{
				Name:   "elmo",
				Type:   "numeric",
				Status: "active",
			},
		},
		MetricLimit: 0,
		Notes:       "",
		Period:      60,
		Status:      "active",
		Target:      "127.0.0.1",
		Timeout:     10,
		Type:        "httptrap",
		Tags:        []string{},
	}

	testBroker = api.Broker{
		CID:  "/broker/1234",
		Name: "test broker",
		Type: "enterprise",
		Details: []api.BrokerDetail{
			api.BrokerDetail{
				CN:           "testbroker.example.com",
				ExternalHost: "",
				ExternalPort: 43191,
				IP:           "127.0.0.1",
				Modules:      []string{"httptrap"},
				Port:         43191,
				Status:       "active",
			},
		},
	}
)

func testCheckServer() *httptest.Server {
	f := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/check_bundle/1234": // handle GET/PUT/DELETE
			switch r.Method {
			case "PUT": // update
				defer r.Body.Close()
				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, string(b))
			case "GET": // get by id/cid
				ret, err := json.Marshal(testCheckBundle)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, string(ret))
			default:
				w.WriteHeader(500)
				fmt.Fprintln(w, "unsupported method")
			}
		case "/check_bundle":
			switch r.Method {
			case "GET": // search
				if strings.HasPrefix(r.URL.String(), "/check_bundle?search=") {
					r := []api.CheckBundle{testCheckBundle}
					ret, err := json.Marshal(r)
					if err != nil {
						panic(err)
					}
					w.WriteHeader(200)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintln(w, string(ret))
				} else {
					w.WriteHeader(200)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintln(w, "[]")
				}
			case "POST": // create
				defer r.Body.Close()
				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, string(b))
			default:
				w.WriteHeader(405)
				fmt.Fprintf(w, "method not allowed %s", r.Method)
			}
		case "/broker":
			switch r.Method {
			case "GET":
				r := []api.Broker{testBroker}
				ret, err := json.Marshal(r)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, string(ret))
			default:
				w.WriteHeader(405)
				fmt.Fprintf(w, "method not allowed %s", r.Method)
			}
		case "/broker/1234":
			switch r.Method {
			case "GET":
				ret, err := json.Marshal(testBroker)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, string(ret))
			default:
				w.WriteHeader(405)
				fmt.Fprintf(w, "method not allowed %s", r.Method)
			}
		case "/check":
			switch r.Method {
			case "GET":
				r := []api.Check{testCheck}
				ret, err := json.Marshal(r)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, string(ret))
			default:
				w.WriteHeader(405)
				fmt.Fprintf(w, "method not allowed %s", r.Method)
			}
		case "/check/1234":
			switch r.Method {
			case "GET":
				ret, err := json.Marshal(testCheck)
				if err != nil {
					panic(err)
				}
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, string(ret))
			default:
				w.WriteHeader(405)
				fmt.Fprintf(w, "method not allowed %s", r.Method)
			}
		default:
			msg := fmt.Sprintf("not found %s", r.URL.Path)
			w.WriteHeader(404)
			fmt.Fprintln(w, msg)
		}
	}

	return httptest.NewServer(http.HandlerFunc(f))
}

func TestUpdateCheck(t *testing.T) {
	server := testCheckServer()
	defer server.Close()

	cm := &CheckManager{
		enabled: true,
	}
	ac := &api.Config{
		TokenApp: "abcd",
		TokenKey: "1234",
		URL:      server.URL,
	}
	apih, err := api.NewAPI(ac)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}
	cm.apih = apih

	newMetrics := make(map[string]*api.CheckBundleMetric)

	t.Log("check manager disabled")
	{
		cm.enabled = false
		cm.UpdateCheck(newMetrics)
	}

	t.Log("no check bundle")
	{
		cm.enabled = true
		cm.checkBundle = nil
		cm.UpdateCheck(newMetrics)
	}

	t.Log("nothing to update (!force metrics, 0 metrics, 0 tags)")
	{
		cm.enabled = true
		cm.checkBundle = &testCheckBundle
		cm.forceCheckUpdate = false
		cm.UpdateCheck(newMetrics)
	}

	newMetrics["test`metric"] = &api.CheckBundleMetric{
		Name:   "test`metric",
		Type:   "numeric",
		Status: "active",
	}

	t.Log("new metric")
	{
		cm.enabled = true
		cm.checkBundle = &testCheckBundle
		cm.forceCheckUpdate = false
		cm.UpdateCheck(newMetrics)
	}

	cm.metricTags = make(map[string][]string)
	cm.metricTags["elmo"] = []string{"cat:tag"}

	t.Log("metric tag")
	{
		cm.enabled = true
		cm.checkBundle = &testCheckBundle
		cm.forceCheckUpdate = false
		cm.UpdateCheck(newMetrics)
	}

}

func TestMakeSecret(t *testing.T) {
	cm := &CheckManager{}

	secret, err := cm.makeSecret()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if secret == "" {
		t.Fatal("Expected a secret, got ''")
	}
}

func TestCreateNewCheck(t *testing.T) {
	server := testCheckServer()
	defer server.Close()

	testURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Error parsing temporary url %v", err)
	}

	hostParts := strings.Split(testURL.Host, ":")
	hostPort, err := strconv.Atoi(hostParts[1])
	if err != nil {
		t.Fatalf("Error converting port to numeric %v", err)
	}

	testBroker.Details[0].ExternalHost = hostParts[0]
	testBroker.Details[0].ExternalPort = hostPort

	cm := &CheckManager{
		enabled:               true,
		checkDisplayName:      "test_dn",
		checkInstanceID:       "test_id",
		checkSearchTag:        api.TagType([]string{"test:test"}),
		checkTarget:           "test",
		brokerMaxResponseTime: time.Duration(time.Millisecond * 500),
		checkType:             "httptrap",
	}

	ac := &api.Config{
		TokenApp: "abcd",
		TokenKey: "1234",
		URL:      server.URL,
	}
	apih, err := api.NewAPI(ac)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}
	cm.apih = apih

	_, _, err = cm.createNewCheck()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

}

func TestInitializeTrapURL(t *testing.T) {
	server := testCheckServer()
	defer server.Close()

	testURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Error parsing temporary url %v", err)
	}

	hostParts := strings.Split(testURL.Host, ":")
	hostPort, err := strconv.Atoi(hostParts[1])
	if err != nil {
		t.Fatalf("Error converting port to numeric %v", err)
	}

	testBroker.Details[0].ExternalHost = hostParts[0]
	testBroker.Details[0].ExternalPort = hostPort

	cm := &CheckManager{
		enabled: false,
		Debug:   false,
		Log:     log.New(os.Stdout, "", log.LstdFlags),
		// Log: log.New(ioutil.Discard, "", log.LstdFlags),
	}

	t.Log("invalid")
	{
		expectedError := errors.New("unable to initialize trap, check manager is disabled")
		err := cm.initializeTrapURL()
		if err == nil {
			t.Fatal("Expected an error")
		}
		if err.Error() != expectedError.Error() {
			t.Fatalf("Expected %v got '%v'", expectedError, err)
		}
	}

	cm.checkSubmissionURL = "http://127.0.0.1:43191/module/httptrap/abc123-a1b2-c3d4-e5f6-123abc/blah"

	t.Log("cm disabled, only submission URL")
	{
		if err := cm.initializeTrapURL(); err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	}

	ac := &api.Config{
		TokenApp: "abcd",
		TokenKey: "1234",
		URL:      server.URL,
	}

	if apih, err := api.NewAPI(ac); err == nil {
		cm.apih = apih
	} else {
		t.Errorf("Expected no error, got '%v'", err)
	}

	cm.trapURL = ""
	cm.enabled = true

	t.Log("cm enabled, submission URL")
	{
		err := cm.initializeTrapURL()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	}

	cm.trapURL = ""
	cm.checkSubmissionURL = ""
	cm.checkID = 1234

	t.Log("cm enabled, check id")
	{
		err := cm.initializeTrapURL()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	}

	cm.trapURL = ""
	cm.checkSubmissionURL = ""
	cm.checkID = 0
	cm.checkTarget = "test_t"
	cm.checkInstanceID = "test_id"
	cm.checkSearchTag = api.TagType([]string{"cat:tag"})
	cm.checkDisplayName = "test_dn"

	t.Log("cm enabled, search [found]")
	{
		if err := cm.initializeTrapURL(); err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	}

	cm.trapURL = ""
	cm.checkSubmissionURL = ""
	cm.checkID = 0
	cm.checkTarget = "foo_t"
	cm.checkInstanceID = "foo_id"
	cm.checkSearchTag = api.TagType([]string{"foo:bar"})
	cm.checkDisplayName = "foo_dn"
	cm.checkType = "httptrap"
	cm.brokerMaxResponseTime = time.Duration(time.Millisecond * 50)

	t.Log("cm enabled, search [not found, create check]")
	{
		err := cm.initializeTrapURL()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	}

	cm.trapURL = ""
	cm.checkSubmissionURL = ""
	cm.checkID = 1234
	cm.checkTarget = "foo_t"
	cm.checkInstanceID = "foo_id"
	cm.checkSearchTag = api.TagType([]string{"foo:bar"})
	cm.checkDisplayName = "foo_dn"
	cm.checkType = "httptrap"

	testCheckBundle.Type = "json:nad"

	t.Log("cm enabled, id, non-httptrap check")
	{
		err := cm.initializeTrapURL()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	}

}
