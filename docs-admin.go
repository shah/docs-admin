package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const VERSION = "1.0.1"

/*
 * Create a structure that implements the Value interface so that directories can be passed
 * in through Flags package
 */

type Directories struct {
	Entries []string
}

func (d *Directories) String() string {
	return strings.Join(d.Entries, ",")
}

func (d *Directories) Set(value string) error {
	d.Entries = append(d.Entries, value)
	return nil
}

type Options struct {
	Agency          string          // the agency to apply if not found in the folder structure
	Component       string          // the component to apply if not found in the folder structure
	SourceDirs      Directories     // the directories we need to search
	Extensions      map[string]bool // a set of extensions we should allow (.pdf .txt, etc)
	FieldsSeparator rune            // the delimeter of the fields in the file name
	HtmlReport      string          // the name of the output file containing the human-readable report
	PhpDataFile     string          // the PHP data file that should be created
	PhpVarName      string          // the PHP variable that the data should be assigned to
	Verbose         bool
}

type WalkedFile struct {
	FileIndex          int
	SourceDirIndex     int
	SourceDir          string
	FullPath           string
	File               os.FileInfo
	FileNameOnlyNoExtn string
	FileExtn           string
	Depth              int
	Ancestors          string
	ContainingDir      string
}

type DocumentInfo struct {
	FileIndex       int
	Agency          string
	Component       string
	YearInOrgStruct string
	YearInFileName  string
	FirstName       string
	LastName        string
	MiddleName      string
	PositionTitle   string
	DocumentType    string
	Emails          []string
	OptionalField   string

	IsValid            bool
	OrgStructParsed    []string
	FieldsParsed       []string
	ValidationMessages []string
}

func getFieldValue(options Options, fields []string, index int, defaultValue string) string {
	// if the field doesn't exist or is blank, return nil
	if len(fields) > index {
		value := strings.TrimSpace(fields[index])
		if len(value) > 0 {
			return value
		}
		return defaultValue
	}
	return defaultValue
}

func (di *DocumentInfo) inspect(options Options, file WalkedFile) {
	di.FileIndex = file.FileIndex

	di.OrgStructParsed = strings.Split(file.Ancestors, string(os.PathSeparator))
	di.Agency = getFieldValue(options, di.OrgStructParsed, 0, options.Agency)
	di.Component = getFieldValue(options, di.OrgStructParsed, 1, options.Component)
	di.YearInOrgStruct = getFieldValue(options, di.OrgStructParsed, 2, "")

	di.FieldsParsed = strings.Split(file.FileNameOnlyNoExtn, string(options.FieldsSeparator))
	di.LastName = getFieldValue(options, di.FieldsParsed, 0, "")
	di.FirstName = getFieldValue(options, di.FieldsParsed, 1, "")
	di.MiddleName = getFieldValue(options, di.FieldsParsed, 2, "")
	di.PositionTitle = getFieldValue(options, di.FieldsParsed, 3, "")
	di.YearInFileName = getFieldValue(options, di.FieldsParsed, 4, "")
	di.DocumentType = getFieldValue(options, di.FieldsParsed, 5, "")

	emailsText := getFieldValue(options, di.FieldsParsed, 6, "")
	if emailsText != "" {
		di.Emails = strings.Split(emailsText, ",")
	}

	di.OptionalField = getFieldValue(options, di.FieldsParsed, 7, "")
}

func (di *DocumentInfo) addMessage(options Options, message string) {
	di.ValidationMessages = append(di.ValidationMessages, message)
	di.IsValid = false
}

func (di *DocumentInfo) validate(options Options, file WalkedFile) {
	di.IsValid = true
	if di.Agency == "" {
		di.addMessage(options, "Agency could not be ascertained from folder structure")
	}
	if di.Component == "" {
		di.addMessage(options, "Component could not be ascertained from folder structure")
	}

	fieldsParsedCount := len(di.FieldsParsed)
	if len(di.FieldsParsed) < 6 {
		di.addMessage(options, fmt.Sprintf("At least six fields are required, only %d were found in the filename", fieldsParsedCount))
	}

	if di.LastName == "" {
		di.addMessage(options, "Last Name was not found in the filename")
	}
	if di.FirstName == "" {
		di.addMessage(options, "First Name was not found in the filename")
	}
	if di.PositionTitle == "" {
		di.addMessage(options, "Position Title was not found in the filename")
	}
	if di.YearInFileName == "" {
		di.addMessage(options, "Year was not found in the filename")
	}
	if di.YearInOrgStruct != "" && di.YearInFileName != "" && di.YearInOrgStruct != di.YearInFileName {
		di.addMessage(options, fmt.Sprintf("The year specified in the folder (%s) is different from the one one specified in the filename (%s)", di.YearInOrgStruct, di.YearInFileName))
	}
	if di.DocumentType == "" {
		di.addMessage(options, "Form (document) type was not found in the filename")
	}
}

type InspectedFile struct {
	File    WalkedFile
	DocInfo DocumentInfo
}

func NewInspectedFile(options Options, file WalkedFile) *InspectedFile {
	result := new(InspectedFile)
	result.File = file
	result.DocInfo.inspect(options, file)
	result.DocInfo.validate(options, file)
	return result
}

type Results struct {
	Options       Options
	DirsWalked    []string
	FilesWalked   []WalkedFile
	Inspected     []InspectedFile
	Errors        []error
	LastFileIndex int
}

func (r *Results) walkSourceDir(sourceDirIndex int, rootPath string, activePath string, depth int) {

	// just the relative path from the root is required
	var ancestors string
	if depth > 0 {
		ancestors = activePath[len(rootPath)+1:]
	} else {
		ancestors = ""
	}
	if r.Options.Verbose {
		fmt.Println("Enter", activePath, depth, ancestors)
	}
	r.DirsWalked = append(r.DirsWalked, activePath)

	entries, err := ioutil.ReadDir(activePath)
	if err != nil {
		r.Errors = append(r.Errors, err)
		return
	}

	for _, entry := range entries {

		if entry.IsDir() {
			r.walkSourceDir(sourceDirIndex, rootPath, filepath.Join(activePath, entry.Name()), depth+1)
		} else {
			r.LastFileIndex++
			fileExtn := filepath.Ext(entry.Name())
			fileNameOnlyNoExtn := entry.Name()[0 : len(entry.Name())-len(fileExtn)]
			walkedFile := WalkedFile{r.LastFileIndex, sourceDirIndex, rootPath, filepath.Join(activePath, entry.Name()), entry, fileNameOnlyNoExtn, fileExtn, depth, ancestors, activePath}
			r.FilesWalked = append(r.FilesWalked, walkedFile)
			if r.Options.Extensions[fileExtn] {
				inspected := NewInspectedFile(r.Options, walkedFile)
				r.Inspected = append(r.Inspected, *inspected)

				if r.Options.Verbose {
					docInfo := inspected.DocInfo
					fmt.Println("Read", docInfo.Agency, docInfo.Component, docInfo.LastName, docInfo.FirstName)
				}
			}
		}
	}
}

func (r *Results) walkSourceDirs() {

	r.DirsWalked = make([]string, 0, 100)
	r.FilesWalked = make([]WalkedFile, 0, 100)
	r.Inspected = make([]InspectedFile, 0, 100)
	r.LastFileIndex = 0

	for index := range r.Options.SourceDirs.Entries {
		dirPath := r.Options.SourceDirs.Entries[index]
		r.walkSourceDir(index, dirPath, dirPath, 0)
	}
}

func (r *Results) createReport(name string, tmplContents string, fileName string) {

	tmpl := template.New(name)
	tmpl, err := tmpl.Parse(tmplContents)
	if err == nil {
		f, err := os.Create(fileName)
		if err == nil {
			err := tmpl.Execute(f, r)
			if err != nil {
				fmt.Println("Error executing "+name+"template: ", err)
			} else {
				fmt.Println("Successfully created", f.Name())
			}
		} else {
			fmt.Println("Error writing "+name+":", err)
		}
		f.Close()
	} else {
		fmt.Println("Error parsing "+name+" template: ", err)
	}

}

func (o *Options) validate() bool {

	usage := flag.Bool("help", false, "Display usage information")

	flag.Parse()

	if *usage {
		flag.Usage()
		return false
	}

	if o.SourceDirs.Entries == nil || len(o.SourceDirs.Entries) == 0 {
		pwd, err := os.Getwd()
		if err == nil {
			o.SourceDirs.Set(pwd)
		} else {
			fmt.Println("Current directory could not be obtained -", err)
			flag.Usage()
			return false
		}
	}

	for _, entry := range o.SourceDirs.Entries {
		f, err := os.Open(entry)
		if err != nil {
			fmt.Println("Folder", entry, "could not be opened -", err)
			flag.Usage()
			return false
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil || !fi.IsDir() {
			fmt.Println(entry, "is not a folder", err)
			flag.Usage()
			return false
		}
	}

	return true
}

func main() {
	var options Options

	flag.Var(&options.SourceDirs, "folder", "The folder to inspect for documents, can be provided multiple times")
	flag.StringVar(&options.Agency, "agency", "", "The agency to use if it's not available in the folder structure")
	flag.StringVar(&options.Component, "component", "", "The component to use if it's not available in the folder structure")
	flag.StringVar(&options.HtmlReport, "report", "report.html", "The file in which to store the HTML report")
	flag.StringVar(&options.PhpDataFile, "phpDataFile", "", "The file in which to store the file information as PHP data")
	flag.StringVar(&options.PhpVarName, "phpVarName", "FILES", "The PHP variable to assign the file data to")

	//options.SourceDirs.Set("C:\\Projects\\MAX-OGE\\docs-generator\\generated-files")
	options.Extensions = map[string]bool{".pdf": true}
	options.FieldsSeparator = ';'

	if options.validate() {
		var results Results
		results.Options = options
		results.walkSourceDirs()
		fmt.Println(results.LastFileIndex, "documents found in", len(results.DirsWalked), "folders.")
		results.createReport("HTML Report", htmlReportTemplate, options.HtmlReport)
		if len(options.PhpDataFile) > 0 {
			results.createReport("PHP Data", phpDataTemplate, options.PhpDataFile)
		}
	}
}

const phpDataTemplate = `
{{.Options.PhpVarName}}_VERSION = ` + VERSION + `;
{{.Options.PhpVarName}} = array(
{{range .Inspected}}
	array( "index" => {{.File.FileIndex}}, 
		   "fullPath" => "{{.File.FullPath}}",
		   "ancestors" => "{{.File.Ancestors}}",
		   "depth" => {{.File.Depth}},
		   "containingDir" => "{{.File.ContainingDir}}",
		{{with .DocInfo}}
		   "agency" => "{{.Agency}}",
		   "component" => "{{.Component}}",
		   "yearInDirName" => "{{.YearInOrgStruct}}",
		   "lastName" => "{{.LastName}}",
		   "firstName" => "{{.FirstName}}",
		   "middleName" => "{{.MiddleName}}",
		   "yearInFileName" => "{{.YearInFileName}}",
		   "docType" => "{{.DocumentType}}",
		   "emails" => array({{range .Emails}}"{{.}}",{{end}})
		   "optional" => "{{.OptionalField}}",
		   "valid" => {{.IsValid}},
		   "errors" => array({{range .ValidationMessages}}"{{.}}",{{end}})
		   ),
		{{end}}
{{end}}
);
`

const htmlReportTemplate = `
<html>
	<head>
		<style>
			body
			{
				font-family: "Lucida Sans Unicode", "Lucida Grande", Sans-Serif;
				font-size: 12px;
				line-height: 1.6em;
			}

			table
			{
				font-family: "Lucida Sans Unicode", "Lucida Grande", Sans-Serif;
				font-size: 12px;
				background: #fff;
				margin: 45px;
				border-collapse: collapse;
				text-align: left;
			}
			table th
			{
				font-size: 14px;
				font-weight: normal;
				color: #039;
				padding: 10px 8px;
				border-bottom: 2px solid #6678b1;
			}
			table td
			{
				color: #669;
				padding: 9px 8px 0px 8px;
			}
			table tbody tr:hover td
			{
				color: #009;
			}
		</style>
		<script>
			function toggleDisplay(elementId) 
			{ 
				var e = document.getElementById(elementId);
				if(e) e.style.display = e.style.display == 'none' ? '' : 'none' 
			}
		</script>
	</head>

	<body>
		<h1>OGE NoPAS Form Submissions Preparation Analysis Results</h1>

		<div>
		Thank you for using the Office of Government Ethics (OGE.gov) NoPAS Forms Submission Preparation
		Analysis Utility version ` + VERSION + `. We have scanned your directories and discovered the following files and associated
		data using the file naming convention rules defined by OGE for file submissions. 
		</div>
		<div>
		Notes:
		<ul>
			<li>If you are looking for a file and do not see it in table below, please look at the 
				<a href="#DirsWalked">Directories Walked</a> section below to see which directories were inspected.</li>
			<li>As you hover (place your mouse) over a file in the table below you will see the original file name.</li>
			<li>When you click on a row in the table below you will be taken to the file details for that file.</li>
		</ul>
		</div>


		<h2>Files Discovered</h2>
		<table id="Inspected">
			<thead>
				<tr class>
					<th>&nbsp;</th>
					<th colspan="3">Folder Structure</th>
					<th colspan="8">File naming convention</th>
					<th colspan="2">Validation</th>
				</tr>
				<tr>
					<th>File</th>
					<th>Agency</th>
					<th>Component</th>
					<th>Year</th>
					<th>Last Name</th>
					<th>First Name</th>
					<th>Middle</th>
					<th>Position Title</th>
					<th>Year</th>
					<th>Form Type</th>
					<th>Emails</th>
					<th>Optional</th>
					<th>Errors?</th>
				</tr>
			</thead>
		{{range .Inspected}}
			<tr style="cursor:hand" onclick="toggleDisplay('FI_DETAILS_{{.File.FileIndex}}')">
				<!-- 
				<td class="ancestors">{{.File.Ancestors}}</td>
				<td class="depth">{{.File.Depth}}</td>
				<td class="nameOnly">{{.File.FileNameOnlyNoExtn}}</td> 
				-->
				<td class="index"><a href="file://{{.File.FullPath}}">{{.File.FileIndex}}</a></td>
				<td class="Agency"><a name="FI_{{.File.FileIndex}}">{{.DocInfo.Agency}}</a></td>
				{{with .DocInfo}}
				<td class="Component">{{.Component}}</td>
				<td class="YearFolder">{{.YearInOrgStruct}}</td>
				<td class="LastName">{{.LastName}}</td>
				<td class="FirstName">{{.FirstName}}</td>
				<td class="Middle">{{.MiddleName}}</td>
				<td class="PositionTitle">{{.PositionTitle}}</td>
				<td class="YearFile">{{.YearInFileName}}</td>
				<td class="DocumentType">{{.DocumentType}}</td>
				<td class="Emails">{{.Emails}}</td>
				<td class="OptionalField">{{.OptionalField}}</td>
				{{end}}
				<td class="IsValid">
					{{if .DocInfo.IsValid}}
					&nbsp;
					{{else}}
					Yes
					{{end}}
				</td>
			</tr>
			<tr id="FI_DETAILS_{{.File.FileIndex}}" style="display:none">
				<td colspan="4">&nbsp;</td>
				<td colspan="9">
					<a title="Click to open and view the file" href="file://{{.File.FullPath}}">{{.File.FullPath}}</a><br/>
					<a title="Click to open the folder in which this file is contained" href="file://{{.File.ContainingDir}}">Open Containing Folder</a>
					| {{.File.File.Size}} bytes | Last Updated: {{.File.File.ModTime}}
					{{if .DocInfo.ValidationMessages}}					
					<div>
					Errors:
					<ol>
						{{range .DocInfo.ValidationMessages}}
						<li>{{.}}</li>
						{{end}}
					</ol>
					</div>
					{{end}}
				</td>
			</tr>
		{{end}}
		</table>

		<h2><a name="DirsWalked">Directories walked</a></h2>
		The following directories were "walked" to see if any files were matched. If a file is missing from the "Files inspected"
		section below, please make sure it's in one of the directories below. If it's not in one of the directories below, then
		it was not inspected or parsed.

		<table id="DirsWalked">		
		{{range .DirsWalked}}
			<tr><td>{{.}}</td></tr>
		{{end}}
		</table>

		<h2>Files walked</h2>
		<table id="FilesWalked">
			<thead>
				<tr>
					<th>File</th>
					<th>Path</th>
					<th>Name</th>
					<th>Bytes</th>
					<th>Date</th>
				</tr>
			</thead>
		{{range .FilesWalked}}
			<tr title="({{.FileIndex}}) {{.FullPath}}" onclick="window.location = '#FI_{{.FileIndex}}'">
				<td class="index"><a name="FW_{{.FileIndex}}">{{.FileIndex}}</a></td>
				<td class="ancestors">{{.Ancestors}}</td>
				<td class="path"><a href="file://{{.FullPath}}">{{.File.Name}}</a></td>
				<td class="size">{{.File.Size}}</td>
				<td class="mtime">{{.File.ModTime}}</td>
			</tr>
		{{end}}
		</table>

		<h1>Troubleshooting</h1>
		<h2>Options Supplied</h2>
		<table id="Options">		
			<thead>
				<tr>
					<th>Name</th>
					<th>Value</th>
				</tr>
			</thead>
			{{with .Options}}
			<tr>
				<td class="name">Agency</td>
				<td class="value">{{.Agency}}</td>
			</tr>
			<tr>
				<td class="name">Component</td>
				<td class="value">{{.Component}}</td>
			</tr>
			<tr>
				<td class="name">Source directories</td>
				<td class="value">{{.SourceDirs.Entries}}</td>
			</tr>
			<tr>
				<td class="name">Extensions</td>
				<td class="value">{{.Extensions}}</td>
			</tr>
			<tr>
				<td class="name">Fields Separator</td>
				<td class="value">{{.FieldsSeparator}}</td>
			</tr>
			<tr>
				<td class="name">Html Report File</td>
				<td class="value">{{.HtmlReport}}</td>
			</tr>
			{{end}}
		</table>

	</body>
</html>
`
