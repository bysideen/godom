package godom

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/GuNanHai/socks"
	"github.com/GuNanHai/toolkit"
)

const (
	// USERAGENT :
	USERAGENT = "User-Agent"
	// PROXY :
	PROXY = "PROXY"
)

// Parser : page string --->  Element
func Parser(body string) Element {
	var element Element
	element.Raw = body
	return element
}

// Find : 加载html源代码为Element后即可使用它查找元素
func (e Element) Find(selectors string) []Element {
	selectorList := generateSelectors(selectors)

	return getElements(selectorList, e)

}

// 从输入的CSS选择器字符串生成并返回  []Selector
func generateSelectors(selectorString string) []Selector {
	// Split 选择器字符串，每个代表一个单独的选择器，并去掉空元素
	selectorArgs := strings.Split(selectorString, " ")
	selectorArgsTemp := []string{}
	for _, each := range selectorArgs {
		if each != "" {
			selectorArgsTemp = append(selectorArgsTemp, each)
		}
	}
	selectorArgs = selectorArgsTemp

	var selectors []Selector

	for _, each := range selectorArgs {
		selector := Selector{}
		// CSS选择器为ID
		if string(each[0]) == "#" {
			selector.Type = "ID"
			each = each[1:]
			if strings.Contains(each, ":") {
				selector.ExtraInfo = each[strings.Index(each, ":"):]
				selector.Value = each[:strings.Index(each, ":")]
			} else if strings.Contains(each, "[") {
				selector.ExtraInfo = each[strings.Index(each, "["):]
				selector.Value = each[:strings.Index(each, "[")]
			} else {
				selector.Value = each
			}
			selectors = append(selectors, selector)
			continue
		}
		//CSS选择器为 CLASS
		if string(each[0]) == "." {
			selector.Type = "CLASS"
			each = each[1:]
			if strings.Contains(each, ":") {
				selector.ExtraInfo = each[strings.Index(each, ":"):]
				selector.Value = each[:strings.Index(each, ":")]
			} else if strings.Contains(each, "[") {
				selector.ExtraInfo = each[strings.Index(each, "["):]
				selector.Value = each[:strings.Index(each, "[")]
			} else {
				selector.Value = each
			}
			selectors = append(selectors, selector)
			continue
		}

		//CSS选择器为 ELEMENT
		selector.Type = "ELEMENT"
		if strings.Contains(each, ":") {
			selector.ExtraInfo = each[strings.Index(each, ":"):]
			selector.Value = each[:strings.Index(each, ":")]
		} else if strings.Contains(each, "[") {
			selector.ExtraInfo = each[strings.Index(each, "["):]
			selector.Value = each[:strings.Index(each, "[")]
		} else {
			selector.Value = each
		}
		selectors = append(selectors, selector)
		continue
	}
	return selectors
}

func getElements(selectors []Selector, e Element) []Element {
	selector := selectors[0]
	newSelectors := selectors[1:]
	var pageTemp Element
	pageTemp.Raw = e.Raw
	pageTemp.Attrs = e.Attrs
	pageTemp.Text = e.Text

	lastIndex := 0
	lastIndexPTR := &lastIndex
	elementList := []Element{}
	elementsFound := []Element{}

	for i := 0; ; i++ {
		pageTemp.Raw = pageTemp.Raw[*lastIndexPTR:]
		element, indexTemp := getElement(selector, pageTemp)

		// fmt.Println(element.Raw)
		// fmt.Println("==========================================", selector.Value, i+1)

		if indexTemp == -1 {
			if len(newSelectors) < 1 {

				elementsFound = append(elementsFound, elementList...)

				return elementsFound
			}
			break
		}
		*lastIndexPTR = indexTemp
		elementList = append(elementList, element)
	}

	for _, element := range elementList {
		elementsFound = append(elementsFound, getElements(newSelectors, element)...)
	}

	return elementsFound
}

// 返回单个CSS选择器中的第一个可能元素 Element,同时返回该Element末尾标签位于传入html中的index============================================================================================
func getElement(selector Selector, e Element) (Element, int) {
	firstIndex := 0
	firstIndex = locatePageFromSingleSelector(e, selector)

	if firstIndex == -1 {
		return Element{}, -1
	}

	upperBody := e.Raw[:firstIndex]

	// fmt.Println(upperBody)
	// fmt.Println("=======================================upper body======================================================")

	reEleStartLoc := regexp.MustCompile(`<`)
	indexList := reEleStartLoc.FindAllStringIndex(upperBody, -1)
	locStart := indexList[len(indexList)-1][0]

	lowerBody := e.Raw[locStart:]
	// fmt.Println(lowerBody)
	// fmt.Println("=======================================lower  body========================================================")

	var tagName string
	if selector.Type == ELEMENT {
		tagName = selector.Value
	} else {
		spaceLoc := strings.Index(lowerBody, " ")
		tagName = lowerBody[1:spaceLoc]
	}

	// fmt.Println("tagName:", tagName)
	// fmt.Println("=====================================tag name===========================================================")

	reEleEndLoc1 := regexp.MustCompile(`<` + tagName)
	reEleEndLoc2 := regexp.MustCompile(`</` + tagName)

	eleStartLocs := reEleEndLoc1.FindAllStringIndex(lowerBody, -1)
	eleEndLocs := reEleEndLoc2.FindAllStringIndex(lowerBody, -1)

	// fmt.Println("被查询标签的开始位置： ", locStart)
	// fmt.Println("开始标签位置集", eleStartLocs)
	// fmt.Println("结束标签位置集", eleEndLocs)

	elementHalfLocList := genElementHalfLocList(eleStartLocs, eleEndLocs)
	sortElementHalfLocList(elementHalfLocList)
	// fmt.Printf("%+v \n", elementHalfLocList)

	closeTagStartLoc, locEnd := getElementEndLoc(elementHalfLocList, lowerBody)
	locEnd = locStart + locEnd

	// 获得元素标签的原始字符串
	elementString := e.Raw[locStart : locEnd+1]
	// fmt.Println(elementString)
	// fmt.Println("================================element string============================================================")

	// 截取出元素标签内的内容，注意：这里的内容如有下级标签，则同样被归为内容Text
	tempIndex := strings.Index(elementString, ">")
	Text := elementString[tempIndex+1 : closeTagStartLoc]

	//截取出元素属性   []Attr
	reAttr := regexp.MustCompile(`\w+=((".*?")|(\w+))`)
	attrFieldString := elementString[:strings.Index(elementString, ">")+1]
	attrList := reAttr.FindAllString(attrFieldString[:len(attrFieldString)-1], -1)
	var attrs []Attr
	for _, attrString := range attrList {
		attr := Attr{}
		attrPair := strings.Split(attrString, "=")

		attr.Name = attrPair[0]
		attr.Value = attrPair[1]
		attrs = append(attrs, attr)
	}

	//生成 Element  类
	var element Element
	element.Raw = elementString
	element.Text = Text
	element.Attrs = attrs

	return element, locEnd + 1
}

// 使用单个选择器从page中搜索到第一个找到的index,并返回
func locatePageFromSingleSelector(e Element, selector Selector) int {
	if selector.Type == ID {
		index := strings.Index(e.Raw, `id="`+selector.Value+`"`)
		if index == -1 {
			// fmt.Println("Error: 网页中不存在该选择器 - ", selector.Type, selector.Value)
			return -1
		}
		return index
	} else if selector.Type == CLASS {

		reLocClass := regexp.MustCompile(`class=(("|')|("|')([^"']*)\s)` + selector.Value + `(("|')|\s([^"']*)("|'))`)
		indexList := reLocClass.FindStringIndex(e.Raw)

		if len(indexList) < 1 {
			// fmt.Println("Error: 网页中不存在该选择器 - ", selector.Type, selector.Value)
			return -1
		}
		return indexList[0]
	} else if selector.Type == ELEMENT {
		index := strings.Index(e.Raw, `<`+selector.Value)
		if index == -1 {
			// fmt.Println("Error: 网页中不存在该选择器 - ", selector.Type, selector.Value)
			return -1
		}
		return index + 1
	}
	fmt.Println("函数locatePageFromSingleSelector有异常出现：")
	return -1
}

//	融合符合条件的标签，及该标签之后的所有同类标签的 半标签（open或者closing tag)的index的集合，这个index集合将用于判断第一个符合条件的标签的closing  tag的index
//  返回 ： []ElementHalfLoc
func genElementHalfLocList(startLocs [][]int, endLocs [][]int) []ElementHalfLoc {
	elementHalfLocs1 := []ElementHalfLoc{}
	elementHalfLocs2 := []ElementHalfLoc{}
	result := []ElementHalfLoc{}
	for _, loc := range startLocs {
		elementHaflLoc := ElementHalfLoc{}
		elementHaflLoc.Loc = loc
		elementHaflLoc.Sign = 0
		elementHalfLocs1 = append(elementHalfLocs1, elementHaflLoc)
	}
	for _, loc := range endLocs {
		elementHaflLoc := ElementHalfLoc{}
		elementHaflLoc.Loc = loc
		elementHaflLoc.Sign = 1
		elementHalfLocs2 = append(elementHalfLocs2, elementHaflLoc)
	}

	result = append(elementHalfLocs1, elementHalfLocs2...)
	return result
}

// 使用冒泡排序，按照html内出现的先后顺序排列  open tag 及 closing tag 的index 的顺序
//  返回 ： []ElementHalfLoc
func sortElementHalfLocList(l []ElementHalfLoc) {
	for i := 0; i < len(l); i++ {
		for j := 0; j < len(l)-i-1; j++ {
			if l[j].Loc[0] > l[j+1].Loc[0] {
				l[j], l[j+1] = l[j+1], l[j]
			}
		}
	}
}

// 根据html内open标签及close标签的特性找到第一个半标签的close标签的index,
//返回close tag </> 的 <  和  > 位置的索引，索引参照标准为传入body
// 注意：这里el内的所有index的参照标准都是传入的body。
func getElementEndLoc(el []ElementHalfLoc, body string) (int, int) {
	flag := 0
	for _, e := range el[1:] {
		if e.Sign == 0 {
			flag = flag + 1
		}
		if e.Sign == 1 {
			flag = flag - 1
			if flag < 0 {
				return e.Loc[0], strings.Index(body[e.Loc[0]:], ">") + e.Loc[0]
			}
		}

	}
	fmt.Println("寻找html元素的结束标签时出现未知错误，该该类标签的所有index如下：")
	fmt.Println(el)
	return 0, -1
}

// ----------------------------------------------------------------------------------------

// Attr : 获取指定属性的值
func (e Element) Attr(attr string) string {
	for _, a := range e.Attrs {
		if strings.Contains(a.Name, attr) {
			return strings.Trim(a.Value, `"`)
		}
	}
	return ""
}

//Get : 不使用代理
func Get(link string, timeout int) Element {
	var e Element

	url, err2 := url.Parse(link)
	toolkit.CheckErr(err2)

	myClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	req, err3 := http.NewRequest("GET", url.String(), nil)
	toolkit.CheckErr(err3)

	req.Header.Set(USERAGENT, RandomUserAgentS())

	resp, err4 := myClient.Do(req)
	if resp == nil {
		// fmt.Println(link, " --  Error: \n", err4)
		// fmt.Println("---------------------------------")

		return Get(link, timeout)
	}
	if resp.StatusCode != 200 || err4 != nil {
		// fmt.Println(link, " -- ", resp.StatusCode, "  Error: \n", err4)
		// fmt.Println("---------------------------------")

		return Get(link, timeout)
	}

	body, err3 := ioutil.ReadAll(resp.Body)
	if err3 != nil {
		// fmt.Println(link, "  网页编码转译失败", err3)
		// fmt.Println("---------------------------------")
		return Get(link, timeout)
	}

	if len(string(body)) < 10 {
		return Get(link, timeout)
	}

	defer resp.Body.Close()

	e.Raw = string(body)

	return e
}

// Fetch ： 访问网页
func Fetch(link string, timeout int) Element {

	var e Element

	proxy := RandomProxy()

	var transport *http.Transport

	if strings.Contains(proxy, "socks") {
		dialSocksProxy := socks.Dial(proxy)
		transport = &http.Transport{Dial: dialSocksProxy}
	} else if strings.Contains(proxy, "https") {
		// 因为无法走https的代理,所以暂时将https转换成http
		temp := strings.Split(proxy, "://")
		proxy = "http://" + temp[1]
		proxyURL, err := url.Parse(proxy)
		toolkit.CheckErr(err)
		transport = &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	} else {
		proxyURL, err := url.Parse(proxy)
		toolkit.CheckErr(err)
		transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	url, err2 := url.Parse(link)
	toolkit.CheckErr(err2)

	myClient := &http.Client{Timeout: time.Duration(timeout) * time.Second, Transport: transport}

	req, err3 := http.NewRequest("GET", url.String(), nil)
	toolkit.CheckErr(err3)

	req.Header.Set(USERAGENT, RandomUserAgentS())

	resp, err4 := myClient.Do(req)
	if resp == nil {
		// fmt.Println(proxy, " --  Error: \n", err4)
		fmt.Println("-------------AccessingFailed--------------------")

		return Fetch(link, timeout)
	}
	if resp.StatusCode != 200 || err4 != nil {
		// fmt.Println(link, " -- ", resp.StatusCode, "  Error: \n", err4)
		// fmt.Println("---------------------------------")
		return Fetch(link, timeout)
	}

	body, err3 := ioutil.ReadAll(resp.Body)
	if err3 != nil {
		// fmt.Println(link, "  网页编码转译失败", err3)
		// fmt.Println("---------------------------------")
		return Fetch(link, timeout)
	}

	if len(string(body)) < 10 {
		return Fetch(link, timeout)
	}

	defer resp.Body.Close()

	e.Raw = string(body)

	return e
}
