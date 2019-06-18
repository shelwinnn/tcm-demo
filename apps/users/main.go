package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type User struct {
	Id    bson.ObjectId `bson:"_id" json:"id"`
	Name  string        `bson:"user" json:"name"`
	Image string        `json:"image"`
}

var globalS *mgo.Session

const (
	MGODB      = "test"
	COLLECTION = "users"
	EGRESSURL  = "http://httpbin.org/anything"
	IMAGEURL   = "https://cdn1.iconfinder.com/data/icons/DarkGlass_Reworked/128x128/apps/user-3.png"
)

func init() {
	var url = os.Getenv("MONGO_DB_URL")
	s, err := mgo.Dial(url)
	if err != nil {
		log.Fatalf("Create Session: %s\n", err)
	}
	globalS = s
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/users", createUser).Methods("POST")
	router.HandleFunc("/users", findUserByName).Methods("GET")
	fmt.Println("starting user service on port 7000")
	http.ListenAndServe(":7000", router)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	result, err := findOneByName(user.Name)
	if err != nil && err != mgo.ErrNotFound {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	} else if err == mgo.ErrNotFound {
		user.Id = bson.NewObjectId()
		if err := insertUser(user); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
	} else {
		result.Image = getImageUrlFromHttpBin()
		responseWithJson(w, http.StatusCreated, result)
		return
	}

	user.Image = getImageUrlFromHttpBin()
	responseWithJson(w, http.StatusCreated, user)
}

func findUserByName(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	name := query.Get("name")
	user, err := findOneByName(name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	user.Image = getImageUrlFromHttpBin()
	responseWithJson(w, http.StatusOK, user)
}

func responseWithJson(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func connect() (*mgo.Session, *mgo.Collection) {
	s := globalS.Copy()
	c := s.DB(MGODB).C(COLLECTION)
	return s, c
}

func insertUser(users ...interface{}) error {
	ms, c := connect()
	defer ms.Close()
	return c.Insert(users...)
}

func findOneByName(name string) (User, error) {
	var result User
	ms, c := connect()
	defer ms.Close()
	err := c.Find(bson.M{"user": name}).Select(nil).One(&result)
	if err != nil {
		return result, err
	}
	return result, nil
}

func getImageUrlFromHttpBin() string {
	image := make(map[string]interface{})
	image["url"] = IMAGEURL
	bytesData, err := json.Marshal(image)
	if err != nil {
		return ""
	}
	httpRequest, err := http.NewRequest("POST", EGRESSURL, bytes.NewReader(bytesData))
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return ""
	}
	defer httpResponse.Body.Close()
	body, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return ""
	}

	if httpResponse.StatusCode != http.StatusOK {
		return ""
	}

	type dataJson struct {
		ImageUrl string `json:"url"`
	}
	type httpBinRsp struct {
		DataJson dataJson `json:"json"`
	}
	resp := &httpBinRsp{}
	err = json.Unmarshal(body, resp)
	if err != nil {
		return ""
	}

	return resp.DataJson.ImageUrl
}
