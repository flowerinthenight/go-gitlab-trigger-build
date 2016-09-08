package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type context struct {
	done  bool
	start time.Time
}

func traceln(v ...interface{}) {
	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)
	fno := regexp.MustCompile(`^.*\.(.*)$`)
	fnName := fno.ReplaceAllString(fn.Name(), "$1")
	m := fmt.Sprintln(v...)
	log.Print("["+fnName+"] ", m)
}

func main() {
	_ref := flag.String("ref", "", "The `[branch]` to build. Branch names, short commit SHAs, and full commit SHAs are also valid.")
	_tag := flag.Bool("tag", false, "Trigger a build with tag.")
	_version := flag.String("version", "", "The `[full-version]` of the build in the format 'major.minor.build.revision'.")
	_token := flag.String("token", "", "The `[token]` for the trigger build authentication.")
	_url := flag.String("url", "", "The `[url]` to send the build trigger.")
	_wait := flag.Bool("wait", true, "Wait for the result by polling the build status until done.")
	_usertoken := flag.String("usrtoken", "", "User's private `[token]` for accessing url's API.")
	flag.Parse()
	if *_ref == "" {
		traceln("No ref/branch provided. See -h option for more information.")
		os.Exit(1)
	}

	if *_token == "" {
		traceln("No trigger token provided. See -h option for more information.")
		os.Exit(1)
	}

	if *_url == "" {
		traceln("No target url. See -h option for more information.")
		os.Exit(1)
	}

	client := &http.Client{}
	data := url.Values{}
	if *_tag {
		if len(*_version) < 1 {
			traceln("No version provided. See -h option for more information.")
			os.Exit(1)
		}

		// Version should match 'major.minor.build.revision' format.
		matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, *_version)
		if !matched {
			traceln("Invalid version format. Should be 'major.minor.build.revision':", *_version)
			os.Exit(1)
		}

		traceln("Starting official build.")
		data.Add("ref", *_ref)
		data.Add("token", *_token)
		data.Add("variables[FULL_VERSION]", *_version)
	} else {
		data.Add("ref", *_ref)
		data.Add("token", *_token)
	}

	r, _ := http.NewRequest("POST", *_url, strings.NewReader(data.Encode()))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, _ := client.Do(r)
	traceln("Response status:", resp.Status)
	if *_wait {
		if *_usertoken == "" {
			traceln("No user private token provided. See -h option for more information.")
			os.Exit(1)
		}

		traceln("Press CTRL+C to terminate.")
	} else {
		return
	}

	// Get base url
	re := regexp.MustCompile(`^http.+projects\/\d+\/`)
	baseurl := re.Find([]byte(*_url))
	traceln("Base URL:", string(baseurl))
	if len(baseurl) == 0 {
		traceln("Unexpected URL format. Should be 'http://<domain>/.../projects/<project-id>/...'.")
		os.Exit(1)
	}

	ids := make(map[string]context)
	var m []map[string]interface{}
	scopes := []string{"running", "pending"}
	for _, scope := range scopes {
		for i := 0; i < 5; i++ {
			traceln("Contacting repository...")
			r, _ := http.NewRequest("GET", string(baseurl)+"builds?scope="+scope, nil)
			r.Header.Add("PRIVATE-TOKEN", *_usertoken)
			resp, err := client.Do(r)
			if err != nil {
				traceln(err)
			} else {
				body, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				err = json.Unmarshal(body, &m)
				if err != nil {
					traceln(err)
				}

				if len(m) > 0 {
					break
				}

				time.Sleep(2 * time.Second)
			}
		}

		for _, items := range m {
			ref, ok := items["ref"]
			if ok {
				if ref == *_ref {
					bld, ok := items["id"]
					if ok {
						str := fmt.Sprintf("%v", bld)
						ids[str] = context{done: false, start: time.Now()}
					}
				}
			}
		}
	}

	if len(ids) < 1 {
		traceln("Cannot detect if build has started.")
		traceln("You can check the status manually in GitLab.")
		os.Exit(1)
	}

	traceln("Active builds:", ids)
	var m2 map[string]interface{}
	for {
		done := true
		for key, val := range ids {
			if val.done {
				continue
			}

			r, _ := http.NewRequest("GET", string(baseurl)+"builds/"+key, nil)
			r.Header.Add("PRIVATE-TOKEN", *_usertoken)
			resp, err := client.Do(r)
			if err != nil {
				traceln(err)
			} else {
				body, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				err = json.Unmarshal(body, &m2)
				if err != nil {
					traceln(err)
				} else {
					ref, ok1 := m2["ref"]
					name, ok2 := m2["name"]
					status, ok3 := m2["status"]
					if ok1 && ok2 && ok3 {
						tr := fmt.Sprintf("%s [%s] build status: %s", ref, name, status)
						traceln(tr)
						if status == "success" || status == "failed" || status == "canceled" {
							ids[key] = context{done: true, start: val.start}
							endtime := time.Now()
							tr := fmt.Sprintf("The %s build took %v to run.", name, endtime.Sub(val.start))
							traceln(tr)
						}
					}
				}
			}
		}

		for _, v := range ids {
			if !v.done {
				done = false
				break
			}
		}

		if done {
			break
		} else {
			time.Sleep(10 * time.Second)
		}
	}
}
