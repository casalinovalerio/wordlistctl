/*
AUTHOR
Valerio Casalino <casalinovalerio.cv@gmail.com>

DESCRIPTION
wordlistctl - Fetch, install and search wordlist archives from websites.
Script to fetch, install, update and search wordlist archives from websites
offering wordlists with more than 6300 wordlists available.

NOTES
This is the first time ever that I'm using Go, so please be kind and make
contributions if you think that something could have been done better,
I'm eager to learn! Thank you!!

LICENSE
GPLv3, this is a derived work from the Black Arch team, more specifically,
as stated in the main document, from sepehrdad@blackarch.org
====> https://github.com/BlackArch/wordlistctl
*/

package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
)

// flag global variables to usage and cli parsing
var (
	DEFAULTSTR = "."
	search     = flag.NewFlagSet("search", flag.ExitOnError)
	fetch      = flag.NewFlagSet("fetch", flag.ExitOnError)
	list       = flag.NewFlagSet("list", flag.ExitOnError)
	listGroup  = list.String("g", DEFAULTSTR, "Specify a group to list: {usernames,passwords,discovery,fuzzing,misc}")
	fetchGroup = fetch.String("g", DEFAULTSTR, "Specify a group to fetch: {usernames,passwords,discovery,fuzzing,misc}")
	fetchBase  = fetch.String("b", "/usr/share/wordlists", "Base directory to store wordlists")
	fetchName  = fetch.String("n", DEFAULTSTR, "The name of the desired wordlist to download")
)

// Default locations of archive.json, which contains the data needed to this program to run
var (
	repoLocation = "/usr/share/wordlistctl/archive.json"
	repoURL      = "https://wl.casalino.xyz/archive.json"
)

// WordlistInfo is made to wrap the JSON info in archive.json
// which is made like so {"name":"...","info":{"url":"...","group":"..."...}
type WordlistInfo struct {
	URL     string `json:"url,omitempty"`
	Group   string `json:"group,omitempty"`
	Size    string `json:"size,omitempty"`
	Updated string `json:"updated,omitempty"`
}

// Wordlist is container for 1 wordlist and its info
type Wordlist struct {
	Name string       `json:"name,omitempty"`
	Info WordlistInfo `json:"info,omitempty"`
}

// Just a wrapper for error messages
func report(msg string) {
	fmt.Fprintln(os.Stderr, "[ERROR]: "+msg)
}

func searchUsage() {
	fmt.Println("==> [SEARCH USAGE]: wordlistctl search 'search-term'")
	search.PrintDefaults()
}

func fetchUsage() {
	fmt.Println("==> [FETCH USAGE]: wordlistctl fetch -[bgn] [ARGS]")
	fetch.PrintDefaults()
}

func listUsage() {
	fmt.Println("==> [LIST USAGE]: wordlistctl list -g [ARGS]")
	list.PrintDefaults()
}

func usage() {
	fmt.Printf("[USAGE]: wordlistctl {search,list,fetch} -[hgb] [ARGS]\n\n")
	searchUsage()
	fmt.Printf("\n")
	listUsage()
	fmt.Printf("\n")
	fetchUsage()
	os.Exit(1)
}

func main() {

	flag.Usage = usage
	search.Usage = searchUsage
	fetch.Usage = fetchUsage
	list.Usage = listUsage
	flag.Parse()

	// If file doesn't exist just re-download it
	if !fileExist(repoLocation) {
		report("Cannot find archive.json (fatal)")
		fmt.Println("Run: \nwget -O", repoLocation, repoURL, "\nTo re-download archive.json")
		os.Exit(2)
	}

	if flag.NArg() < 1 {
		report("Expected at least a command")
		usage()
	}

	// Making this check before we load the wordlist archive into memory
	if os.Args[1] != "search" && os.Args[1] != "list" && os.Args[1] != "fetch" {
		report("Please input a valid mode")
		usage()
	}

	// Preloading the wordlists
	wordlistArray := getAllWordlists(repoLocation)

	switch os.Args[1] {
	case "search":
		if len(os.Args) != 3 {
			report("Provide search term")
			searchUsage()
			os.Exit(1)
		}
		searchRoutine(os.Args[2], wordlistArray)
	case "fetch":
		fetch.Parse(os.Args[2:])
		if *fetchName == DEFAULTSTR {
			if *fetchGroup == DEFAULTSTR {
				report("You should choose either a group or a name...")
				os.Exit(2)
			}
			fetchMulti(wordlistArray, *fetchGroup, *fetchBase)
		} else {
			if *fetchGroup != DEFAULTSTR {
				report("You shouldn't choose bot a group and a name...")
				os.Exit(2)
			}
			fetchOne(wordlistArray, *fetchName, *fetchBase)
		}
	case "list":
		list.Parse(os.Args[2:])
		listRoutine(wordlistArray, *listGroup)
	default:
		// I'm unreachable
	}
}

// This function checks if the specified file exists and
// that's not a directory, so that we can use it
// https://golangcode.com/check-if-a-file-exists/
func fileExist(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Reads the repo file and returns an array with all the
// wordlists info
func getAllWordlists(repoName string) []Wordlist {
	var wordlists []Wordlist
	wordlistFile, _ := ioutil.ReadFile(repoName)
	err := json.Unmarshal([]byte(wordlistFile), &wordlists)
	if err != nil {
		panic(err)
	}
	return wordlists
}

// Converts the array to a Map to make things easier for me...
// Then I'll optimize, I promise
func convertWordlistToMap(arrayed []Wordlist) map[string]WordlistInfo {
	mapped := make(map[string]WordlistInfo)
	for _, wordlist := range arrayed {
		mapped[wordlist.Name] = wordlist.Info
	}
	return mapped
}

// DownloadFile will download a url and store it in local filepath.
// It writes to the destination file as it downloads it, without
// loading the entire file into memory.
// https://progolang.com/how-to-download-files-in-go/
func downloadFile(url string, filepath string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// Decompress tar archive .tar.gz
// https://stackoverflow.com/a/30813114
func decompress(targetdir string, reader io.ReadCloser) error {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		target := path.Join(targetdir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			setAttrs(target, header)
			break

		case tar.TypeReg:
			w, err := os.Create(target)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, tarReader)
			if err != nil {
				return err
			}
			w.Close()

			setAttrs(target, header)
			break

		default:
			log.Printf("unsupported type: %v", header.Typeflag)
			break
		}
	}
	return nil
}

func setAttrs(target string, header *tar.Header) {
	os.Chmod(target, os.FileMode(header.Mode))
	os.Chtimes(target, header.AccessTime, header.ModTime)
}

func downloadAndExtract(url string, downloadPath string, finalPath string) {
	downloadFile(url, downloadPath)
	reader, err := os.Open(downloadPath)
	if err != nil {
		fmt.Println("error")
	}
	decompress(finalPath, reader)
}

func printInfo(wordlist Wordlist) {
	fmt.Println(wordlist.Name, " (", wordlist.Info.Size, ") [", wordlist.Info.Updated, "] ")
}

func searchRoutine(term string, wordlists []Wordlist) {
	for _, wordlist := range wordlists {
		matched, err := regexp.MatchString(term, wordlist.Name)
		if err != nil {
			report("Error in regexpr... Not sure what it means")
		}
		if matched {
			printInfo(wordlist)
		}
	}
}

func fetchOne(wordlistArray []Wordlist, name string, basedir string) {
	wordlistMap := convertWordlistToMap(wordlistArray)
	downloadPath := os.TempDir() + "/" + name + ".tar.gz"
	finalPath := basedir + "/" + name
	result, ok := wordlistMap[name]
	if ok {
		downloadAndExtract(result.URL, downloadPath, finalPath)
	} else {
		report("No wordlist found with that name")
	}
}

func fetchMulti(wordlistArray []Wordlist, group string, basedir string) {
	for _, wordlist := range wordlistArray {
		if wordlist.Info.Group == group {
			downloadAndExtract(wordlist.Info.URL, os.TempDir()+"/"+wordlist.Name+"tar.gz", basedir+"/"+wordlist.Name)
		}
	}
}

func listRoutine(wordlistArray []Wordlist, group string) {
	for _, wordlist := range wordlistArray {
		if group == wordlist.Info.Group || group == DEFAULTSTR {
			printInfo(wordlist)
		}
	}
}
