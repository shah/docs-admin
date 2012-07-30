package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	sourceDirs      []string        // the directories we need to search
	extensions      map[string]bool // a set of extensions we should allow (.pdf .txt, etc)
	fieldsSeparator rune            // the delimeter of the fields in the file name
	htmlReport      string          // the name of the output file containing the human-readable report
	jsonResult      string          // the file in which to store the Results JSON representation
}

type WalkedFile struct {
	file               os.FileInfo
	fileNameOnlyNoExtn string
	fileExtn           string
	depth              int
	ancestors          string
}

type DocumentInfo struct {
	agency        string
	component     string
	year          string
	firstName     string
	lastName      string
	middleName    string
	positionTitle string
	documentType  string
	emails        []string
	optionalField string

	isValid            bool
	validationMessages []string
}

func (di *DocumentInfo) inspect(options Options, file WalkedFile) {
	orgs := strings.Split(file.ancestors, string(os.PathSeparator))
	if len(orgs) > 0 {
		di.agency = orgs[0]
	}
	if len(orgs) > 1 {
		di.component = orgs[1]
	}

	fields := strings.Split(file.fileNameOnlyNoExtn, string(options.fieldsSeparator))

	if len(fields) > 0 {
		di.lastName = fields[0]
	}
	if len(fields) > 1 {
		di.firstName = fields[1]
	}
	if len(fields) > 2 {
		di.middleName = fields[2]
	}
	if len(fields) > 3 {
		di.middleName = fields[3]
	}
	if len(fields) > 4 {
		di.positionTitle = fields[4]
	}
	if len(fields) > 5 {
		di.year = fields[5]
	}
	if len(fields) > 6 {
		di.emails = strings.Split(fields[6], ",")
	}
	if len(fields) > 7 {
		di.optionalField = fields[5]
	}
}

type InspectedFile struct {
	file    WalkedFile
	docInfo DocumentInfo
}

func NewInspectedFile(options Options, file WalkedFile) *InspectedFile {
	result := new(InspectedFile)
	result.file = file
	result.docInfo.inspect(options, file)
	return result
}

type Results struct {
	dirsWalked  []string
	filesWalked []WalkedFile
	inspected   []InspectedFile
	errors      []error
}

func (r *Results) walkSourceDir(options Options, rootPath string, activePath string, depth int) {

	// just the relative path from the root is required
	var ancestors string
	if depth > 0 {
		ancestors = activePath[len(rootPath)+1:]
	} else {
		ancestors = ""
	}
	fmt.Println("Enter", activePath, depth, ancestors)
	r.dirsWalked = append(r.dirsWalked, activePath)

	entries, err := ioutil.ReadDir(activePath)
	if err != nil {
		r.errors = append(r.errors, err)
		return
	}

	for _, entry := range entries {

		if entry.IsDir() {
			r.walkSourceDir(options, rootPath, filepath.Join(activePath, entry.Name()), depth+1)
		} else {
			fileExtn := filepath.Ext(entry.Name())
			fileNameOnlyNoExtn := entry.Name()[0 : len(entry.Name())-len(fileExtn)]
			walkedFile := WalkedFile{entry, fileNameOnlyNoExtn, fileExtn, depth, ancestors}
			r.filesWalked = append(r.filesWalked, walkedFile)
			if options.extensions[fileExtn] {
				inspected := NewInspectedFile(options, walkedFile)
				r.inspected = append(r.inspected, *inspected)
				fmt.Println("Read", inspected.docInfo.agency, inspected.docInfo.component, inspected.docInfo.lastName, inspected.docInfo.firstName)
			}
		}
	}
}

func (r *Results) walkSourceDirs(options Options) {

	r.dirsWalked = make([]string, 0, 100)
	r.filesWalked = make([]WalkedFile, 0, 100)
	r.inspected = make([]InspectedFile, 0, 100)

	for index := range options.sourceDirs {
		dirPath := options.sourceDirs[index]
		r.walkSourceDir(options, dirPath, dirPath, 0)
	}
}

func (o *Options) validate() {
	fmt.Println("TODO: Validate Options!\n", o)
}

func main() {
	var options Options

	options.sourceDirs = []string{"C:\\Projects\\MAX-OGE\\docs-generator\\generated-files"}
	options.extensions = map[string]bool{".pdf": true}
	options.fieldsSeparator = ';'
	options.htmlReport = "report.html"
	options.jsonResult = "result.json"

	options.validate()

	var results Results
	results.walkSourceDirs(options)
	b, err := json.Marshal(results.inspected)
	os.Stdout.Write(b)
	fmt.Print("Error: ", err)
}
