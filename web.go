package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
)

const projectID = "res-log"

func getMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", homeElm)
	mux.HandleFunc("/r", receive)
	mux.HandleFunc("/r/", receive)
	mux.Handle("/l", http.NotFoundHandler())
	mux.HandleFunc("/l/", resourcesView)
	mux.HandleFunc("/cron/daily", dailyView)
	mux.Handle("/task/process_hook", authDecor(http.HandlerFunc(processHookView)))
	mux.Handle("/task/save_resource", authDecor(http.HandlerFunc(saveResourceView)))
	mux.Handle("/task/purge_before", authDecor(http.HandlerFunc(purgeBeforeView)))
	mux.Handle("/task/purge_step", authDecor(http.HandlerFunc(purgeStepView)))
	return mux
}

var elmTmpl = template.Must(template.ParseFiles("templates/index_elm.html"))

func homeElm(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := elmTmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func receive(w http.ResponseWriter, r *http.Request) {
	err := processBody(r)
	if err != nil {
		log.Printf("failed to process request with error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Add("X-Application-SHA256", AppKey256)
	fmt.Fprintf(w, "OK")
}

func processBody(r *http.Request) error {
	mac := hmac.New(sha256.New, []byte(cfg.AppKey)) //used later to verify signature

	rdr, err := pack(io.TeeReader(io.LimitReader(r.Body, 8*1024*1024), mac)) //8MB arbitrary limit
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(rdr)
	if err != nil {
		return err
	}

	//now verify the HMAC
	//decode message mac
	messageMAC, err := hex.DecodeString(r.Header.Get("X-Gapi-Signature"))
	if err != nil {
		return err
	}

	expectedMAC := mac.Sum(nil)
	if !hmac.Equal(messageMAC, expectedMAC) {
		return fmt.Errorf("Unexpected X-Gapi-Signature received: %s", r.Header.Get("X-Gapi-Signature"))
	}

	//log.Printf("processed data long %d", len(data))
	processHookLater(r.Context(), data)
	return nil
}

type hookDataAttr struct {
	ID   interface{} `json:"id"`
	Href string      `json:"href"`
}

type hookStruct struct {
	EventType string        `json:"event_type"`
	Resource  string        `json:"resource"`
	Created   string        `json:"created"`
	Data      *hookDataAttr `json:"data"`
}

func processHook(ctx context.Context, in io.Reader) error {
	r, err := unpack(in)
	if err != nil {
		log.Printf("abandon processHook failed to unpack data: %v", err)
		return nil
	}
	var events []*hookStruct
	dec := json.NewDecoder(r)
	err = dec.Decode(&events)
	if err != nil {
		log.Printf("abandon processHook failed to decode json: %v", err)
		return nil
	}
	for _, v := range events {
		saveResourceLater(ctx, v)
		/*
			task, err := saveResourceLater.Task(v)
			if err != nil {
				return err
			}
			// set retry options for the task
			// min/max back off of 5 mins means
			// we will retry every 5 minutes
			// 20 times
			// after which point it will be abandoned
			ropt := taskqueue.RetryOptions{
				RetryLimit: 20,
				MinBackoff: 5 * time.Minute,
				MaxBackoff: 5 * time.Minute,
			}
			task.RetryOptions = &ropt
			_, err = taskqueue.Add(ctx, task, "")
			if err != nil {
				return err
			}*/
	}
	return nil
}

//Resource is our basic model representing the REST resource that we save to datastore
type Resource struct {
	URI       string `datastore:"Uri"`
	Type      string `datastore:"Type"`
	HookDate  string `datastore:",noindex"`
	Data      []byte `datastore:",noindex"`
	FetchDate time.Time
	Sha1      string `datastore:",noindex"`
}

//JSONResource is the same as Resource but more suitable for serializing
type JSONResource struct {
	FetchDate string          `json:"fetchdate"`
	HookDate  string          `json:"hookdate"`
	Sha1      string          `json:"sha1"`
	Data      json.RawMessage `json:"resource"`
}

//jsLayout is for formatting dates
const jsLayout = "2006-01-02T15:04:05Z"

//CountingWriter keeps track of the number of bytes written
type CountingWriter struct {
	Written int
	w       io.Writer
}

//Write implements io.Writer
func (cw *CountingWriter) Write(data []byte) (int, error) {
	num, err := cw.w.Write(data)
	cw.Written = cw.Written + num
	return num, err
}

//WriteString convenience wrapper
func (cw *CountingWriter) WriteString(str string) (int, error) {
	return cw.Write([]byte(str))
}

//NewCountingWriter returns new instance of counting writer
func NewCountingWriter(out io.Writer) *CountingWriter {
	return &CountingWriter{
		0, out,
	}
}

//WriteAsJSON writes this resource to Writer as JSON
func (r *Resource) WriteAsJSON(out io.Writer) (int, error) {
	jsr := JSONResource{
		FetchDate: r.FetchDate.Format(jsLayout),
		HookDate:  r.HookDate,
		Sha1:      r.Sha1,
	}
	if len(r.Data) > 0 {
		dr, err := gzip.NewReader(bytes.NewBuffer(r.Data))
		if err != nil {
			return 0, err
		}
		jsr.Data, err = ioutil.ReadAll(dr)
		if err != nil {
			return 0, err
		}
		if err := dr.Close(); err != nil {
			return 0, err
		}
	}
	outbuf := NewCountingWriter(out)
	err := json.NewEncoder(outbuf).Encode(&jsr)
	return outbuf.Written, err
}

//MarshalJSON implements the json.Marshaller
func (r *Resource) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := r.WriteAsJSON(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func saveResource(c context.Context, hook *hookStruct) error {
	var uriBuf bytes.Buffer
	uriBuf.WriteString(hook.Resource)
	uriBuf.WriteString("/")
	switch t := hook.Data.ID.(type) {
	case int:
		uriBuf.WriteString(strconv.Itoa(t))
	case float64:
		uriBuf.WriteString(strconv.Itoa(int(t)))
	case string:
		uriBuf.WriteString(t)
	default:
		return fmt.Errorf("Unexpected type for Hook.Data.ID: %T", t)
	}
	//fetch the resource
	client := http.DefaultClient
	req, err := http.NewRequest("GET", hook.Data.Href, nil)
	if err != nil {
		log.Printf("failed to build GET request for: %s", hook.Data.Href)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Application-Key", cfg.AppKey)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("failed to fetch: %s", hook.Data.Href)
		return err
	}
	defer resp.Body.Close()

	//first reader json verif
	origBuf := new(bytes.Buffer)
	firstR := io.TeeReader(io.LimitReader(resp.Body, 8*1024*1024), origBuf) //arbitrary 8MB limit
	//before packing calc the sha1 - second reader
	shaw := sha1.New()
	shar := io.TeeReader(firstR, shaw)
	//pack the response returns a final reader
	pr, err := pack(shar)
	if err != nil {
		log.Printf("failed to pack: %s", hook.Data.Href)
		return err
	}
	pdata, err := ioutil.ReadAll(pr)
	if err != nil {
		log.Printf("failed to read packed: %s", hook.Data.Href)
		return err
	}
	if len(pdata) > MaxDataStoreByteSize {
		log.Printf(
			"compressed resource is too large %d abandon: %s",
			len(pdata),
			hook.Data.Href)
		return nil
	}
	//now verify that this is ok json
	var someJSON map[string]interface{}
	dec := json.NewDecoder(origBuf)
	if derr := dec.Decode(&someJSON); derr != nil {
		log.Printf(
			"failed to properly decode json so abandon %s: %v",
			hook.Data.Href, derr)
		return nil
	}
	//create and save
	r := Resource{
		URI:       uriBuf.String(),
		Type:      hook.Resource,
		HookDate:  hook.Created,
		Data:      pdata,
		FetchDate: time.Now().UTC(),
		Sha1:      hex.EncodeToString(shaw.Sum(nil))}

	dsClient, err := datastore.NewClient(c, projectID)
	if err != nil {
		log.Printf("unable to create Datastore client %v", err)
		return err
	}

	_, err = dsClient.Put(c, datastore.IncompleteKey("resource", nil), &r)
	if err != nil {
		log.Printf("unable to store resource %#v", r)
		return err
	}
	return nil
}

//MaxDataStoreByteSize is the largest size a blob in DS can have
const MaxDataStoreByteSize = 1048576

func getURLPart(prefix, urlpath string, idx int) (r string) {
	if len(urlpath) <= len(prefix) {
		return
	}
	s := urlpath[len(prefix):]
	p := strings.Split(s, "/")
	if len(p) > idx {
		r = p[idx]
	}
	return
}

func isPrivate(res string) bool {
	private := []string{"documents", "refunds", "payments", "agents"}
	r := strings.ToLower(strings.TrimSpace(res))
	for _, v := range private {
		if r == v {
			return true
		}
	}
	return false
}

//MaxRespSize is maximum response size we are willing to return
const MaxRespSize = 30 * 1024 * 1024 //30MB arbitrary arrived at via 500 errors

func resourcesView(w http.ResponseWriter, r *http.Request) {
	//enable the CORS preflight wonder used by browsers
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, HEAD")
	for _, value := range r.Header["Access-Control-Request-Headers"] {
		w.Header().Add("Access-Control-Allow-Headers", value)
	}
	w.Header().Add("Access-Control-Max-Age", "3600")
	//end of CORS
	//response content type header
	w.Header().Add("content-type", "application/json")

	//query
	restype := getURLPart("/l/", r.URL.Path, 0)
	resid := getURLPart("/l/", r.URL.Path, 1)
	if restype == "" {
		http.Error(w, "missing resource type or ID", http.StatusBadRequest)
		return
	}
	if isPrivate(restype) {
		http.Error(w, "Not Authorized", http.StatusForbidden)
		return
	}
	c := r.Context()
	dsClient, err := datastore.NewClient(c, projectID)
	if err != nil {
		log.Printf("Failed to create a datastore client %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if resid == "" {
		//no ID passed get the most recent id
		resid, err = getRecentIDForResource(c, dsClient, restype)
		if err != nil {
			log.Printf("Failed to query most recent ID for resource %s %v", restype, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	str := bytes.NewBufferString(restype)
	str.WriteString("/")
	str.WriteString(resid)
	q := datastore.NewQuery("resource").Filter("Uri =", str.String()).Order("-FetchDate")
	//iterate query and write it to response up to a limit
	var (
		totalBytes int64
		isFirst    = true
	)
	t := dsClient.Run(c, q)
	out := bufio.NewWriter(w)
	out.WriteString("[")
	for {
		var res Resource
		_, err := t.Next(&res)
		if err == iterator.Done {
			break
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !isFirst {
			out.WriteString(",")
		} else {
			isFirst = false
		}
		size, err := res.WriteAsJSON(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		totalBytes = totalBytes + int64(size)
		if totalBytes >= MaxRespSize {
			break
		}
	}
	out.WriteString("]")
	out.Flush()
}

//PurgeInput represents the data struct for purge operation
type PurgeInput struct {
	Before string
}

func purgeBefore(ctx context.Context, when time.Time, encCursor string) (err error) {
	var (
		stop      bool
		keys      []*datastore.Key
		newCursor string
	)
	q := datastore.NewQuery("resource").Filter("FetchDate <", when).KeysOnly()
	if encCursor != "" {
		cursor, err := datastore.DecodeCursor(encCursor)
		if err == nil {
			q = q.Start(cursor)
		}
	} else {
		log.Printf("Starting purge of anything older than %v", when)
	}

	// Iterate over the results.
	dsClient, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("unable to create Datastore client %v", err)
		return
	}

	t := dsClient.Run(ctx, q)
	for i := 0; i < 100; i++ {
		key, err := t.Next(nil)
		if err == iterator.Done {
			stop = true
			break
		}
		if err != nil {
			log.Printf("fetching next Key: %v", err)
			return err
		}
		//do something with it
		keys = append(keys, key)
	}

	// Get updated cursor and store it for next time.
	if cursor, err := t.Cursor(); err == nil {
		newCursor = cursor.String()
	}

	err = dsClient.DeleteMulti(ctx, keys)
	if err != nil {
		log.Printf("trouble with multi delete: %v", err)
		return err
	}
	if !stop {
		purgeStepLater(ctx, when, newCursor)
	}
	return nil
}

func getRecentIDForResource(ctx context.Context, client *datastore.Client, resource string) (string, error) {
	q := datastore.NewQuery("resource").
		Filter("Type =", resource).
		Order("-FetchDate").
		Limit(1)
	var resources []Resource
	keys, err := client.GetAll(ctx, q, &resources)
	if err != nil {
		return "", err
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("Query for recent ID had no results")
	}
	return strings.Replace(resources[0].URI, resource+"/", "", 1), nil
}

func dailyView(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	t := time.Now().UTC().Add(-31 * 24 * time.Hour)
	purgeBeforeLater(ctx, t)
	w.Header().Add("content-type", "application/json")
	fmt.Fprintf(w, "\"OK\"")
}
