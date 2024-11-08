package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/caiflower/common-tools/pkg/tools"
)

func TestReg(t *testing.T) {
	matchStr := `\/\{[a-zA-Z][a-zA-Z0-9_]*[a-zA-Z]\}`
	matchStr1 := `/([a-zA-Z0-9!#@$%^&()|/*/._-]*)`

	reg := tools.MatchReg("/tests/{testId}", matchStr)
	fmt.Println(reg)

	find := tools.RegFind("/tests/{testId}/tests2/{test2Id}", matchStr)

	for _, str := range find {
		fmt.Println(str[2 : len(str)-1])
	}

	regstr := tools.RegReplace("/tests/{testId}/tests2/{test2Id}", matchStr, matchStr1)
	fmt.Println(regstr)

	clean := tools.RegReplace("/tests/{testId}/tests2/{test2Id}", matchStr, "")
	fmt.Println(clean)
	splits := strings.Split(clean, "/")
	splits = splits[1:]

	path := "/tests/_-.$#@!#@$%^&*()|/tests2/ters"
	reg = tools.MatchReg(path, regstr)
	fmt.Println(reg)

	find = tools.RegFind(path, regstr)
	for _, v := range find {
		fmt.Println(v)
	}
	if reg {
		for i, str := range splits {
			path = strings.TrimPrefix(path, "/"+str+"/")
			fmt.Println(path)

			for j := 0; j < len(path); j++ {
				if i+1 < len(splits) {
					if strings.HasPrefix(path[j:], "/"+splits[i+1]) {
						fmt.Println(path[:j])
						path = path[j:]
						continue
					}
				} else {
					j++
				}
			}
		}
	}

	reg = tools.MatchReg("/tests/{testId}/tests2/{test2Id}", matchStr)
	fmt.Println(reg)

	reg = tools.MatchReg("/tests/{testId}/action:do", matchStr)
	fmt.Println(reg)

	reg = tools.MatchReg("/tests/{testId_}/action:do", matchStr)
	fmt.Println(reg)

	reg = tools.MatchReg("/tests/action:do", matchStr)
	fmt.Println(reg)
}
