package main

import (
	"os"
	"io"
	"fmt"
	"flag"
	"bytes"
	"strings"
	"strconv"
//	"regexp"
	"html"
	"io/ioutil"
	"text/template"
	"encoding/xml"
	"github.com/russross/blackfriday"
)

var mode string
var modeList []string

func pFail() {
	if err := recover(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}

type getId func() []byte

func h3Factory() (getId, getId)  {
	var id uint64 = 0
	tmp := []byte("id-" + mode + "-")
	buf := make([][]byte, 0)
	return func() []byte {
		id++
		buf = append(buf, strconv.AppendUint(tmp, id, 10))
		return buf[id - 1]
	}, func() []byte {
		return buf[id - 1]
	}
}

type title []byte

func (t *title) init() {
	*t = make([]byte, 0)
}

func (t *title) addMore(bts []byte) {
	if *t == nil {
		*t = make([]byte, 0)
	}
	if len(bts) > 0 {
		*t = append(*t, bts...)
	}
}

func (t *title) has() bool {
	if len(*t) > 0 {
		return true
	}
	return false
}

func (t *title) reset() {
	if *t == nil {
		*t = make([]byte, 0)
	}
	*t = []byte(*t)[0:0]
}

func (t *title) String() string{
	return string(*t)
}

type a struct {
	Href string `xml:"href,attr"`
	Title3 string `xml:",chardata"`
}

type li struct {
//	Class string `xml:"class,attr"`
	A a `xml:"a"`
}

type listMenu struct {
	XMLName xml.Name `xml:"ul"`
	Lis []li `xml:"li"`
}

func (l *listMenu) init() {
	if l.Lis == nil {
		l.Lis = make([]li, 0)
	}
}

type mdContent struct {
	title3 title
	listMenu listMenu
	ListMenu string
	Content string
	Align string
	Color string
}

func (md *mdContent) init() *mdContent {
	md.title3.init()
	md.listMenu.init()
	if mode == "none" {
		md.Align = "margin: 0 auto;"
		md.Color = "#ffffff"
	} else {
		md.Align = "float: right;"
		md.Color = "#efefef"
	}
	return md
}

func (md *mdContent) addToUl(id []byte) {
	a := &a{string(append([]byte{'#'}, id...)), strings.Trim(md.title3.String(), " \t\n")}
	md.listMenu.Lis = append(md.listMenu.Lis, li{*a})
	md.title3.reset()
}

func (md *mdContent) fillContentXML(output []byte) error {
	buf := bytes.NewBuffer(output)
	d := xml.NewDecoder(buf)
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	fdh := false
	getNewId, getLastId := h3Factory()
	contBuf := bytes.NewBuffer(make([]byte, 0, len(output) * 2))
	for token, err := d.RawToken();err != io.EOF; token, err = d.RawToken() {
		if  err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if mode != "none" && t.Name.Local	== mode {
				fdh = true
				md.title3.reset()
				t.Attr = append(t.Attr, xml.Attr{xml.Name{"", "id"}, string(getNewId())})
			}
			contBuf.WriteString("<")
			contBuf.WriteString(html.EscapeString(t.Name.Local))
			for _, a := range t.Attr {
				contBuf.WriteString(fmt.Sprintf(" %s=\"%s\"", a.Name.Local, a.Value))
			}
			contBuf.WriteString(">")
		case xml.EndElement:
			if mode != "none" && t.Name.Local == mode {
				fdh = false
				md.addToUl(getLastId())
			}
			contBuf.WriteString(fmt.Sprintf("</%s>", html.EscapeString(t.Name.Local)))
		case xml.CharData:
			if fdh {
				md.title3.addMore(t)
			}
			contBuf.WriteString(html.EscapeString(string(t)))
		case xml.ProcInst:
			contBuf.WriteString(fmt.Sprintf("<?%s %s>", t.Target, t.Inst))
		case xml.Directive:
			contBuf.WriteString(fmt.Sprintf("<!%s>", t))
		case xml.Comment:
			contBuf.WriteString(fmt.Sprintf("<!--%s-->", t))
		default:
			contBuf.WriteString("INVALID TOKEN")
		}
	}
	md.Content = contBuf.String()

	if mode != "none" {
		listMenu, err := xml.MarshalIndent(md.listMenu, "", "  ")
		if err != nil {
			return err
		}
		md.ListMenu = string(listMenu)
	}
	return nil
}

/*
func (md *mdContent) fillContentREG(output []byte) error {
	reg := regexp.MustCompile(`<h1>(.*)</h1>`)
	found := reg.FindSubmatch(output)
	if len(found) >= 2 {
		md.Title1 = found[1]
	}
//	fmt.Printf("%s\n", md.Title1)
	
	reg = regexp.MustCompile(`<h2>(.*)</h2>`)
	found = reg.FindSubmatch(output)
	if len(found) >= 2 {
		md.Title2 = found[1]
	}
//	fmt.Printf("%s\n", md.Title2)	
	
	reg = regexp.MustCompile(`<h3>(.*)(</h3>)`)
	ind := reg.FindSubmatchIndex(output)
	fmt.Println(ind)
	dst := []byte{}
	dst = reg.Expand(dst, []byte("$1"), output, ind)
	fmt.Println(dst, string(dst))
//	output = reg.ReplaceAllFunc(output, func (old []byte) []byte {
//		fmt.Printf("%s, %s\n", string(old), reg.SubexpNames()[1])

//		return old
//	})

	return nil
}
*/

func (md *mdContent) fillContent(output []byte) error {
	err := md.fillContentXML(output)
	if err != nil {
		return err
	}

/*	err := md.fillContentREG(output)
	if err != nil {
		return err
	}
*/
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: md2min [-nav=h2] name.md\n  name.md: markdown file name\n")
	flag.PrintDefaults()
}

func init() {
	modeList = []string{"none", "h1", "h2", "h3", "h4", "h5", "h6"}
	flag.StringVar(&mode, "nav", "none", `navigate level ["none", "h1", "h2", "h3", "h4", "h5", "h6"]`)
}

func main() {
	defer pFail()

	flag.Parse()

	i := 0
	for ; i<len(modeList); i++ {
		if modeList[i] == mode {
			break
		}
	}
	
	if flag.NArg() < 1 || i >= len(modeList) {
		usage()
		return
	}

	name := flag.Args()[0]
	file, err := os.Open(name)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	input, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	
	output := blackfriday.MarkdownBasic(input)
	md := new(mdContent).init()
	err = md.fillContent(output)
	if err != nil {
		panic(err)
	}

	newname := strings.TrimRight(name, ".md") + ".html"
	outfile, err := os.Create(newname)
	if err != nil {
		panic(err)
	}
	defer outfile.Close()

	tmpl, err := template.New("tmpl").Parse(templContent)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(outfile, md)
	if err != nil {
		panic(err)
	}
}
