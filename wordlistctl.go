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
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"text/tabwriter"

	"golang.org/x/sys/unix"

	"github.com/alexflint/go-arg"
	"github.com/h2non/filetype"
)

// Default locations of archive.json, which contains the data needed to this program to run and other
// important variables
var (
	version      = "v1.1-beta"
	binaryName   = path.Base(os.Args[0])
	repoLocation = "/usr/share/wordlistctl/archive.json"
	repoURL      = "https://raw.githubusercontent.com/casalinovalerio/wordlistctl/main/archive.json"
)

// SearchCmd is a struct for search subcommand (go-arg)
type SearchCmd struct {
	Term string `arg:"positional" help:"Search term that will be parsed as regexp"`
}

// ListCmd is a struct for list subcommand (go-arg)
type ListCmd struct {
	Group string `arg:"-g,--group" help:"Specify a group to list: {usernames,passwords,discovery,fuzzing,misc}"`
}

// FetchCmd is a struct for fetch subcommand (go-arg)
type FetchCmd struct {
	Group string `arg:"-g,--group" help:"specify a group to fetch: {usernames,passwords,discovery,fuzzing,misc}"`
	Base  string `arg:"-b,--base" help:"base directory to store wordlists" default:"/usr/share/wordlists/"`
	Name  string `arg:"positional" help:"the name of the desired wordlist to download"`
}

// Var for main command (go-arg)
var args struct {
	Fetch   *FetchCmd  `arg:"subcommand:fetch" help:"fetch a wordlist"`
	List    *ListCmd   `arg:"subcommand:list" help:"list the wordlists that are available"`
	Search  *SearchCmd `arg:"subcommand:search" help:"search a particular wordlist"`
	Version bool       `arg:"-v,--version" help:"display binary version"`
}

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

func main() {

	// Terminate if no subcommand is specified
	if p := arg.MustParse(&args); p.Subcommand() == nil {
		if args.Version {
			fmt.Printf("(%s) Version: %s\n", binaryName, version)
			os.Exit(0)
		}
		p.Fail("Missing subcommand")
	}

	// If file doesn't exist just re-download it
	if !fileExist(repoLocation) {
		report("Cannot find archive.json (fatal)")
		fmt.Println("Run: \nwget -O", repoLocation, repoURL, "\nTo re-download archive.json")
		os.Exit(2)
	}

	// Preloading the wordlists
	wordlistArray := getAllWordlists(repoLocation)

	switch {
	case args.Search != nil:
		if args.Search.Term == "" {
			report("No search term provided")
			os.Exit(2)
		} else {
			searchRoutine(args.Search.Term, wordlistArray)
		}
	case args.List != nil:
		listRoutine(wordlistArray, args.List.Group)
	case args.Fetch != nil:
		if !isWritable(args.Fetch.Base) {
			report("You don't have permissions to write in that dir")
			os.Exit(2)
		}
		if args.Fetch.Name == "" {
			if args.Fetch.Group == "" {
				report("You should choose either a group or a name...")
				os.Exit(2)
			}
			fetchMulti(wordlistArray, args.Fetch.Group, args.Fetch.Base)
		} else {
			if args.Fetch.Group != "" {
				report("You shouldn't choose bot a group and a name...")
				os.Exit(2)
			}
			fetchOne(wordlistArray, args.Fetch.Name, args.Fetch.Base)
		}
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

// Checker to see if we have the right permission to write
// in the folder we want to write
func isWritable(path string) bool {
	return unix.Access(path, unix.W_OK) == nil
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

// Decompress gzip archive
func decompressGzip(targetdir string, archive string) string {
	reader, err := os.Open(archive)
	if err != nil {
		fmt.Println("error")
	}
	defer reader.Close()

	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return ""
	}
	defer gzReader.Close()

	target, err := os.Create(path.Join(targetdir, gzReader.Name))
	if err != nil {
		return ""
	}

	if _, err := io.Copy(target, gzReader); err != nil {
		return ""
	}

	if os.Remove(archive) != nil {
		report("It was impossible to clean")
	}

	return target.Name()
}

func decompressTar(targetdir string, archive string) string {
	reader, err := os.Open(archive)
	if err != nil {
		fmt.Println("error")
	}
	defer reader.Close()

	// Decompress from tarball to final
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return ""
		}
		target := path.Join(targetdir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, os.FileMode(header.Mode))
			if err != nil {
				return ""
			}
			os.Chmod(target, os.FileMode(header.Mode))
			os.Chtimes(target, header.AccessTime, header.ModTime)
			break
		case tar.TypeReg:
			w, err := os.Create(target)
			if err != nil {
				return ""
			}
			_, err = io.Copy(w, tarReader)
			if err != nil {
				return ""
			}
			w.Close()
			os.Chmod(target, os.FileMode(header.Mode))
			os.Chtimes(target, header.AccessTime, header.ModTime)
			return target

		default:
			log.Printf("unsupported type: %v", header.Typeflag)
			break
		}
	}
	return ""
}

// To move files no matter of the partitions
func moveFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("Couldn't open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("Couldn't open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("Writing to output file failed: %s", err)
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("Failed removing original file: %s", err)
	}
	return nil
}

func downloadAndExtract(url string, downloadPath string, finalPath string) {
	fmt.Println("==> Downloading: \n", url)
	downloadFile(url, downloadPath)
	fmt.Println("Done!")

	// Creating folder (group)
	os.Mkdir(finalPath, os.ModePerm)

	buf, _ := ioutil.ReadFile(downloadPath)

	if filetype.IsType(buf, filetype.Types["gz"]) {
		fmt.Println("==> Extracting...")
		intermediate := decompressGzip(os.TempDir(), downloadPath)
		buf, _ := ioutil.ReadFile(intermediate)
		if filetype.IsType(buf, filetype.Types["tar"]) {
			final := decompressTar(finalPath, intermediate)
			if os.Remove(intermediate) != nil {
				report("It was impossible to clean")
			}
			if final != finalPath {
				report("final and finalPath are not the same")
			}
		} else {
			err := moveFile(intermediate, path.Join(finalPath, path.Base(intermediate)))
			if err != nil {
				panic(err)
			}
		}
	} else {
		err := moveFile(downloadPath, path.Join(finalPath, path.Base(downloadPath)))
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Wordlist saved to ", finalPath, "\nIt was smooth, wasn't it?")
}

func printInfo(wordlist Wordlist) {
	w := new(tabwriter.Writer)

	// minwidth, tabwidth, padding, padchar, flags
	w.Init(os.Stdout, 35, 8, 0, '\t', 0)
	defer w.Flush()
	fmt.Fprintf(w, ">"+wordlist.Name+"\t("+wordlist.Info.Size+")\t["+wordlist.Info.Updated+"]\n")
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
	result, ok := wordlistMap[name]
	if ok {
		downloadAndExtract(result.URL, path.Join(os.TempDir(), name), path.Join(basedir, result.Group))
	} else {
		report("No wordlist found with that name")
	}
}

func fetchMulti(wordlistArray []Wordlist, group string, basedir string) {
	for _, wordlist := range wordlistArray {
		if wordlist.Info.Group == group {
			downloadAndExtract(wordlist.Info.URL, path.Join(os.TempDir(), wordlist.Name), path.Join(basedir, wordlist.Info.Group))
		}
	}
}

func listRoutine(wordlistArray []Wordlist, group string) {
	for _, wordlist := range wordlistArray {
		if group == wordlist.Info.Group || group == "" {
			printInfo(wordlist)
		}
	}
}
