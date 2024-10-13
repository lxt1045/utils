package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/tag"
	"github.com/markuskont/datamodels"
	"github.com/markuskont/go-sigma-rule-engine"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

var (
	// dir = "D:/project/go/src/github.com/lxt1045/sigma_rule/run_236_new"
	dir = "D:/project/go/src/github.com/lxt1045/sigma_rule/right"
)

func Test_sigma11(t *testing.T) {
	ctx := context.Background()
	file := "D:/project/go/src/github.com/lxt1045/sigma_rule/right_1/1001301010.yml"
	// file := "D:/download/1004501011.yml"
	// file := "D:/download/2010502004.yml"

	bsObj := `{
		"log_id": 1825473211280965634,
		"platform": "windows",
		"agent_id": 2,
		"time": 1700104412094930700,
		"event_time": 1700104411121750300,
		"mark": -127,
		"source": "Microsoft-Windows-Sysmon/Operational",
		"session_id": -127,
		"username": "DESKTOP-NL0NE6O\\admin",
		"parent_username": "DESKTOP-NL0NE6O\\admin",
		"parent_cmdline": "wesensor.exe install",
		"parent_path": "C:\\Users\\admin\\Downloads\\agent-10.1.1.200_202311161014\\wesensor.exe",
		"description": "Service Control Manager Configuration Tool",
		"product": "Microsoft® Windows® Operating System",
		"company": "Microsoft Corporation",
		"event_id": 1,
		"logon_id": 1962153,
		"logon_guid": "{81c15ea6-00e4-6555-a9f0-1d0000000000}",
		"pid": 16936,
		"name": "sc.exe",
		"path": "C:\\Windows\\System32\\sc.exe",
		"cmdline": "sc start Wesensord",
		"cwd": "C:\\Users\\admin\\Downloads\\agent-10.1.1.200_202311161014\\",
		"euid": -127,
		"egid": -127,
		"on_disk": -127,
		"ppid": 13676,
		"pgroup": 0,
		"virtual_process": -127,
		"upid": 1825473211280965634,
		"uppid": 1825473211280982018,
		"flags": -127,
		"exit_code": -127,
		"token_elevation_status": -127,
		"socket": -127,
		"family": -127,
		"initiated": -127,
		"local_port": -127,
		"remote_port": -127,
		"size": -127,
		"mtime": -127,
		"ctime": -127,
		"is_executable": -127,
		"file_version": "10.0.22621.1 (WinBuild.160101.0800)",
		"original_filename": "sc.exe",
		"sha256": "SHA1:ECE3EFDB9BAC287FA85088E49264AF099EB32313,MD5:FF2A4319FA5531F0D7B98DBBA9ABBD4A,SHA256:67CBE3A7B9C9B7A2E9FB10A33C1A9A3FD297B6F1598AEA374BD9B5D2DEE352EC,IMPHASH:42DADBD60BBC453DD1773FDC089E19B7",
		"source_pid": -127,
		"target_pid": -127,
		"signed": -127,
		"new_thread_id": -127,
		"tty": "1",
		"rule_name": "technique_id:T1031,technique_name:Modify Existing Service"
	 }`
	var input Log
	err := json.Unmarshal([]byte(bsObj), &input)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := NewOneRuleset(ctx, file, input)
	if err != nil {
		t.Fatal(err)
	}
	ruleIDs := rs.Eval2(&input, nil)

	{
		bs, _ := json.Marshal(&ruleIDs)
		t.Logf("results:%+v", string(bs))
	}

	t.Log("end...")
}
func Test_sigma(t *testing.T) {
	ctx := context.Background()
	file := "D:/project/go/src/github.com/lxt1045/sigma_rule/right/01001201001.yml"
	// file := "D:/download/2010502004.yml"

	bsObj := `{
		"log_id": 1822718767126659080,
		"platform": "windows",
		"agent_id": 8,
		"time": 1697539135314726000,
		"event_time": 1697539135021119900,
		"source": "Microsoft-Windows-Sysmon/Operational",
		"username": "SYSTEM",
		"parent_username": "lixiantu\\86189",
		"parent_cmdline": "C:\\Windows\\Explorer.EXE",
		"parent_path": "C:\\Windows\\explorer.exe",
		"description": "Beyond Compare",
		"product": "Beyond Compare",
		"company": "Scooter Software",
		"event_id": 1,
		"logon_guid": "{554e3258-f0be-6527-6a46-080000000000}",
		"pid": 27172,
		"name": "BCompare.exe",
		"path": "C:\\Program Files\\Beyond Compare 4\\BCompare.exe",
		"cmdline": "\"C:\\Program Files\\Beyond Compare 4\\BCompare.exe\" ",
		"cwd": "C:\\Program Files\\Beyond Compare 4\\",
		"ppid": 9308,
		"upid": 1822718767126691848,
		"uppid": 1822716598168354824,
		"file_version": "4.4.6.27483",
		"original_filename": "BCompare.exe",
		"sha256": "SHA1=C1A9044FE63DB93050C20A3D961D7A2D45042115,MD5=32012A5AAAE41BEDAC3167411C426382,SHA256=C4DB10508A30327B8019813D09C6274A1060747058DB6FB64CBBF6544E93E6DC,IMPHASH=72FA3C3B727F8724D5D64BE2B6C80EF1",
		"new_thread_id": 44292,
		"tty": "1",
		"rule_name": "technique_id=T1204,technique_name=User Execution"
	}`
	var input Log
	dec := json.NewDecoder(bytes.NewBufferString(bsObj))
	dec.UseNumber()
	if err := dec.Decode(&input); err != nil {
		t.Fatal(err)
	}

	rs, err := NewOneRuleset(ctx, file, input)
	if err != nil {
		t.Fatal(err)
	}
	ruleIDs := rs.Eval(&input, nil)

	{
		bs, _ := json.Marshal(&ruleIDs)
		t.Logf("results:%+v", string(bs))
	}

	t.Log("end...")
}

func Test_sigma_func(t *testing.T) {
	ctx := context.Background()
	file := "D:/project/go/src/github.com/lxt1045/sigma_rule/right/10001302002.yml"
	// file := "D:/download/2010502004.yml"

	bsObj := `{
		"log_id": 1822718767126659080,
		"platform": "windows",
		"agent_id": 8,
		"time": 1697539135314726000,
		"event_time": 1697539135021119900,
		"source": "Microsoft-Windows-Sysmon/Operational",
		"username": "SYSTEM",
		"parent_username": "lixiantu\\86189",
		"parent_cmdline": "C:\\Windows\\Explorer.EXE",
		"parent_path": "C:\\Windows\\system32\\CMD.exe",
		"description": "Beyond Compare",
		"product": "Beyond Compare",
		"company": "Scooter Software",
		"event_id": 1,
		"logon_guid": "{554e3258-f0be-6527-6a46-080000000000}",
		"pid": 27172,
		"name": "BCompare.exe",
		"path": "C:\\Program Files\\Beyond Compare 4\\BCompare.exe",
		"cmdline": "C:\\Program Files\\Beyond Compare 4\\BCompare.exe",
		"cwd": "C:\\Program Files\\Beyond Compare 4\\",
		"ppid": 9308,
		"upid": 1822718767126691848,
		"uppid": 1822716598168354824,
		"file_version": "4.4.6.27483",
		"original_filename": "BCompare.exe",
		"sha256": "SHA1=C1A9044FE63DB93050C20A3D961D7A2D45042115,MD5=32012A5AAAE41BEDAC3167411C426382,SHA256=C4DB10508A30327B8019813D09C6274A1060747058DB6FB64CBBF6544E93E6DC,IMPHASH=72FA3C3B727F8724D5D64BE2B6C80EF1",
		"new_thread_id": 44292,
		"tty": "1",
		"rule_name": "technique_id=T1204,technique_name=User Execution"
	}`
	var input Log
	dec := json.NewDecoder(bytes.NewBufferString(bsObj))
	dec.UseNumber()
	if err := dec.Decode(&input); err != nil {
		t.Fatal(err)
	}

	rs, err := NewOneRuleset(ctx, file, input)
	if err != nil {
		t.Fatal(err)
	}
	ruleIDs := rs.Eval(&input, nil)

	{
		bs, _ := json.Marshal(&ruleIDs)
		t.Logf("results:%+v", string(bs))
	}

	t.Log("end...")
}

func Test_sigma2(t *testing.T) {
	data := []byte(`title: 尝试使用显式凭据进行登录。
id: 1003
status: 1 
decisioncenter: 1
description: 票据登录的一部分
detection:
  selection1:
    event_id: '4648'
  selection2:
    event_data: 'test'
  selection3:
    event_data1: 'test'
  selection4:
    event_data2: 'test'
  condition: selection1 or selection2 and selection3 or selection4
level: INFO
tags:
  - T1590
  `)
	var rule sigma.Rule
	err := yaml.Unmarshal(data, &rule)
	if err != nil {
		t.Fatal(err)
	}
	ruleHandle := sigma.RuleHandle{
		Path:         "",
		Rule:         rule,
		NoCollapseWS: false,
		Multipart: func() bool {
			return !bytes.HasPrefix(data, []byte("---")) && bytes.Contains(data, []byte("---"))
		}(),
	}
	tree, err := sigma.NewTree(ruleHandle)
	if err != nil {
		t.Fatal(err)
	}
	var obj datamodels.Map = map[string]any{
		"event_id":    "4648",
		"event_data":  "test1",
		"event_data1": "test",
		"event_data2": "test1",
	}
	result, ok := tree.Eval(obj)
	if !ok {
		t.Fatal("!ok")
	}
	if result.ID != "1003" {
		t.Error("has ho 1003")
	}
	{
		bs, _ := json.Marshal(&result)
		t.Logf("results:%+v", string(bs))
	}

	t.Log("end...")
}

func Test_parseCondition(t *testing.T) {
	mt := map[string][]SigmaBranch[Log]{
		"selection1": {
			{SelectionKey: "selection1", OR: false},
		},
		" selection1 ": {
			{SelectionKey: "selection1", OR: false},
		},
		" selection1  or selection2": {
			{SelectionKey: "selection1", OR: true, Not: false},
			{SelectionKey: "selection2", OR: true, Not: false},
		},
		" selection1  and selection2": {
			{SelectionKey: "selection1", OR: false, Not: false},
			{SelectionKey: "selection2", OR: false, Not: false},
		},
		"not selection1": {
			{SelectionKey: "selection1", OR: false, Not: true},
		},
		"not selection1 or not selection2": {
			{SelectionKey: "selection1", OR: true, Not: true},
			{SelectionKey: "selection2", OR: true, Not: true},
		},
		"not selection1 and not selection2": {
			{SelectionKey: "selection1", OR: false, Not: true},
			{SelectionKey: "selection2", OR: false, Not: true},
		},
		"(not selection1 and not selection2)": {
			{Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection1", OR: false, Not: true},
				{SelectionKey: "selection2", OR: false, Not: true},
			}},
		},
		" ( not selection1 and not selection2 ) ": {
			{Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection1", OR: false, Not: true},
				{SelectionKey: "selection2", OR: false, Not: true},
			}},
		},
		"not (not selection1 and not selection2)": {
			{Not: true, Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection1", OR: false, Not: true},
				{SelectionKey: "selection2", OR: false, Not: true},
			}},
		},
		"not(not selection1 and not selection2)": {
			{Not: true, Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection1", OR: false, Not: true},
				{SelectionKey: "selection2", OR: false, Not: true},
			}},
		},
		"(not selection1 and not selection2) and (not selection3 and not selection4)": {
			{OR: false, Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection1", OR: false, Not: true},
				{SelectionKey: "selection2", OR: false, Not: true},
			}},
			{OR: false, Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection3", OR: false, Not: true},
				{SelectionKey: "selection4", OR: false, Not: true},
			}},
		},
		"(not selection1 and not selection2)and(not selection3 and not selection4)": {
			{OR: false, Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection1", OR: false, Not: true},
				{SelectionKey: "selection2", OR: false, Not: true},
			}},
			{OR: false, Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection3", OR: false, Not: true},
				{SelectionKey: "selection4", OR: false, Not: true},
			}},
		},
		" selection1 and selection4 and (selection2 or selection3) and not selection5 and not Image ": {
			{SelectionKey: "selection1", OR: false},
			{SelectionKey: "selection4", OR: false},
			{Sons: []SigmaBranch[Log]{
				{SelectionKey: "selection2", OR: true},
				{SelectionKey: "selection3", OR: true},
			}},
			{SelectionKey: "selection5", OR: false, Not: true},
			{SelectionKey: "Image", OR: false, Not: true},
		},
	}

	selections := map[string]Selection{
		"selection1": {},
		"selection2": {},
		"selection3": {},
		"selection4": {},
		"selection5": {},
		"Image":      {},
	}
	t.Run("parseCondition-one", func(t *testing.T) {
		k := "not selection1 and not selection2"
		v := []SigmaBranch[Log]{
			{SelectionKey: "selection1", OR: false, Not: true},
			{SelectionKey: "selection2", OR: false, Not: true},
		}
		// 这里不是 nil 的话，后面 assert.Equal 会报错。
		cs, err := (*SigmaRule[Log])(nil).ParseCondition(k, selections)
		if err != nil {
			t.Fatalf("k:%s, v:%+v, err:%+v", k, v, err)
		}

		if !assert.Equal(t, v, cs) {
			bs, err := json.Marshal(&cs)
			if err != nil {
				t.Fatal(err)
			}
			t.Log(k, ":/n", string(bs))
		}
	})

	t.Run("parseCondition", func(t *testing.T) {
		for k, v := range mt {
			cs, err := (*SigmaRule[Log])(nil).ParseCondition(k, selections)
			if err != nil {
				t.Fatalf("k:%s, v:%+v, err:%+v", k, v, err)
			}

			if !assert.Equal(t, v, cs) {
				bs, err := json.Marshal(&cs)
				if err != nil {
					t.Fatal(err)
				}
				t.Log(k, ":/n", string(bs))
			}
		}
	})

	// 错误
	mterr := map[string]string{
		"not and selection1": `and after not`,
		// "selection1 and selection2 or selection3": `"()" must be added between "and"/"or"`,
	}

	t.Run("parseCondition-error", func(t *testing.T) {
		p := &SigmaRule[Log]{
			yml: &SigmaYml[Log]{
				RuleID: 1111,
			},
		}
		selections := make(map[string]Selection)
		for k, v := range mterr {
			_, err := p.ParseCondition(k, selections)
			// if !assert.EqualError(t, err, v) {
			// 	t.Log(k, ":", v)
			// }
			if !strings.Contains(err.Error(), v) {
				t.Fatalf("err:%+v, v:%s", err, v)
			}
		}
	})
}

func hasID(ids []int64, id int64) bool {
	for _, id0 := range ids {
		if id == id0 {
			return true
		}
	}
	return false
}

func mapToStruct[T any](m map[string]interface{}) (info T, err error) {
	tis, err := tag.NewTagInfos(info, "json")
	if err != nil {
		return
	}
	for k, v := range m {
		// key := tagMap[k]
		// if key == "" {
		// 	err = fmt.Errorf("tagMap[k] is nil")
		// 	return
		// }
		key := k
		ti, ok := tis[key]
		if !ok {
			err = UtilsUnexpected.Clonef("key not exist:%s", key)
			return
		}
		offset := ti.Offset

		switch ti.BaseKind {
		case reflect.String:
			fElemP := StructElemPointer[string, T]
			pstr := fElemP(&info, offset)
			*pstr = v.(string)
		case reflect.Int64:
			fElemP := StructElemPointer[int64, T]
			pstr := fElemP(&info, offset)
			ok := false
			*pstr, ok = v.(int64)
			if !ok {
				*pstr, err = v.(json.Number).Int64()
				if err != nil {
					err = UtilsUnexpected.Clonef("type not support:%s, key:%s, err:%+v", ti.BaseKind, key, err)
					return
				}
			}
		case reflect.Struct:
			fElemP := StructElemPointer[int64, T]
			pstr := fElemP(&info, offset)
			*pstr, err = v.(json.Number).Int64()
			if err != nil {
				err = UtilsUnexpected.Clonef("type not support:%s, key:%s, err:%+v", ti.BaseKind, key, err)
				return
			}
		default:
			err = UtilsUnexpected.Clonef("type not support:%s, key:%s", ti.BaseKind, key)
			return
		}
	}
	return
}

func Test_Eval(t *testing.T) {
	// dir := "D:/project/go/src/github.com/lxt1045/sigma_rule/run_236_new"
	// dir = "D:/project/go/src/github.com/lxt1045/sigma_rule/right"
	dir := "D:/project/go/src/github.com/lxt1045/sigma_rule/from_db/"
	ctx := context.Background()
	bs := []byte(`{
		"log_id": 1820954305453212865,
		"platform": "windows",
		"agent_id": 11,
		"event_id": 1,
		"time": 1695895852709671500,
		"event_time": 1695894164033431500,
		"source": "Microsoft-Windows-Sysmon/Operational",
		"session_id": 1,
		"username": "lixiantu\\86189",
		"parent_cmdline": "C:\\Windows\\system32\\cmd.exe /d /s /c \"git show --format=%h[githd-fs]%cr[githd-fs]%b[githd-fs] --stat 02b9e611899ff4366c67c878cd0bdf8e55d5831f\"",
		"parent_path": "C:\\Windows\\System32\\QQ1\\Bin\\QQ.exe",
		"description": "Git for Windows",
		"product": "Git",
		"company": "The Git Development Community",
		"pid": 28504,
		"cwd": "d:\\project\\go\\src\\gitlab.wecode.com\\dev\\agent\\",
		"ppid": 17040,
		"file_version": "2.40.1.windows.1",
		"new_thread_id": 7664,
		"extra1": "git  show --format=%%h[githd-fs]%%cr[githd-fs]%%b[githd-fs] --stat 02b9e611899ff4366c67c878cd0bdf8e55d5831f",
		"extra2": "C:\\Program Files\\Git\\cmd\\git.exe",
		"extra4": "CommandLinexxxxxxx",
		"original_filename": "dsquery.exe"
	}`)

	t.Run("split", func(t *testing.T) {
		rs, err := NewRulesetByDir(ctx, dir, Log{})
		if err != nil {
			t.Fatal(err)
		}

		rsLinux, rsWin, setNetwork := rs.SplitByPlatform()

		var input Log
		err = json.Unmarshal(bs, &input)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = rsLinux, setNetwork

		ruleIDs := rsWin.Eval(&input, nil)
		ok := hasID(ruleIDs, 1001301004)
		if !ok {
			t.Fatal("has no 1001301004")
		}
		t.Logf("ruleIDs:%+v", ruleIDs)

		input.EventId = 5145
		ruleIDs = rs.Eval(&input, nil)
		ok = hasID(ruleIDs, 8001101003)
		if !ok {
			t.Fatal("has ho 8001101003")
		}
		t.Logf("ruleIDs:%+v", ruleIDs)

		t.Log("end...")
	})

	t.Run("my", func(t *testing.T) {
		rs, err := NewRulesetByDir(ctx, dir, Log{})
		if err != nil {
			t.Fatal(err)
		}

		var input Log
		err = json.Unmarshal(bs, &input)
		if err != nil {
			t.Fatal(err)
		}

		ruleIDs := rs.Eval(&input, nil)
		ok := hasID(ruleIDs, 1001301004)
		if !ok {
			t.Fatal("has no 1001301004")
		}
		t.Logf("ruleIDs:%+v", ruleIDs)

		input.EventId = 5145
		ruleIDs = rs.Eval(&input, nil)
		ok = hasID(ruleIDs, 8001101003)
		if !ok {
			t.Fatal("has ho 8001101003")
		}
		t.Logf("ruleIDs:%+v", ruleIDs)

		t.Log("end...")
	})

	t.Run("sigma", func(t *testing.T) {
		ruleset, err := sigma.NewRuleset(sigma.Config{
			Directory:       []string{dir},
			NoCollapseWS:    false,
			FailOnRuleParse: false,
			FailOnYamlParse: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		var obj datamodels.Map
		if err := json.Unmarshal(bs, &obj); err != nil {
			t.Fatal(err)
		}
		results, ok := ruleset.EvalAll(obj)
		if !ok || len(results) == 0 {
			t.Fatalf("!ok || len(results) == 0, ok:%v, len(results):%+v", ok, len(results))
		}
		ok = hasRule(results, "1001301004")
		if !ok {
			t.Fatal("has ho 1001301004")
		}
		{
			bs, _ := json.Marshal(&results)
			t.Logf("results:%+v", string(bs))
		}

		obj["event_id"] = "5145"
		results, ok = ruleset.EvalAll(obj)
		if !ok || len(results) == 0 {
			t.Error("!ok || len(results) == 0 ")
		}
		ok = hasRule(results, "8001101003")
		if !ok {
			t.Fatal("has ho 8001101003")
		}
		{
			bs, _ := json.Marshal(&results)
			t.Logf("results:%+v", string(bs))
		}

		t.Log("end...")
	})
}

func hasRule(results sigma.Results, id string) bool {
	for _, r := range results {
		if r.ID == id {
			return true
		}
	}
	return false
}

func Test_Eval_BenchMark(t *testing.T) {
	// dir := "D:/project/go/src/github.com/lxt1045/sigma_rule/run_236_new"
	bs, err := os.ReadFile("./test.json")
	if err != nil {
		t.Fatal(err)
	}

	const (
		M = 1
		N = 1000 * 1000 * 1000
		T = 10
	)
	t.Logf("M:%d, N:%d, T:%d", M, N, T)

	t.Run("my", func(t *testing.T) {
		var wg sync.WaitGroup
		var msgSuss int64
		ctx := context.TODO()
		ctx, _ = context.WithTimeout(ctx, time.Second*T)
		showQps(ctx, &msgSuss)

		rs, err := NewRulesetByDir(ctx, dir, Log{})
		if err != nil {
			t.Fatal(err)
		}

		var m map[string]interface{}
		dec := json.NewDecoder(bytes.NewBuffer([]byte(bs)))
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}

		input, err := mapToStruct[Log](m)
		if err != nil {
			t.Fatal(err)
		}
		ruleIDs := rs.Eval(&input, nil)
		t.Logf("ruleIDs:%+v", ruleIDs)

		buf := make([]int64, 0, 16)
		for j := 0; j < M; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < N; i++ {
					ruleIDs = rs.Eval(&input, buf)
					atomic.AddInt64(&msgSuss, 1)

					select {
					case <-ctx.Done():
						i = N + 1
					default:
					}
				}
			}()
		}
		wg.Wait()
		t.Log("end...")
	})

	t.Run("sigma", func(t *testing.T) {
		var wg sync.WaitGroup
		var msgSuss int64
		ctx := context.TODO()
		ctx, _ = context.WithTimeout(ctx, time.Second*T)
		showQps(ctx, &msgSuss)

		ruleset, err := sigma.NewRuleset(sigma.Config{
			Directory:       []string{dir},
			NoCollapseWS:    false,
			FailOnRuleParse: false,
			FailOnYamlParse: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		var obj datamodels.Map
		if err := json.Unmarshal(bs, &obj); err != nil {
			t.Fatal(err)
		}
		results, ok := ruleset.EvalAll(obj)
		if !ok || len(results) == 0 {
			t.Fatal("!ok || len(results) == 0 ")
		}
		{
			bs, _ := json.Marshal(&results)
			t.Logf("results:%+v", string(bs))
		}

		for j := 0; j < M; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < N; i++ {
					if results, ok := ruleset.EvalAll(obj); !ok || len(results) == 0 {
						t.Error("!ok || len(results) == 0 ")
					}
					atomic.AddInt64(&msgSuss, 1)

					select {
					case <-ctx.Done():
						i = N + 1
					default:
					}
				}
			}()
		}

		wg.Wait()
		t.Log("end...")
	})
}

/*
go test -benchmem -run=^$ -bench ^Benchmark_Eval$ github.com/lxt1045/broker/output/sigma/engine -count=1 -v -cpuprofile cpu.prof -c
go test -benchmem -run=^$ -bench ^Benchmark_Eval$ github.com/lxt1045/broker/output/sigma/engine -count=1 -v -memprofile cpu.prof -c
go tool pprof sigma.test.exe cpu.prof
go tool pprof -http=:8080 cpu.prof
*/

// TODO: fEval 函数调用转成循环，避免嵌套调用损耗

func Benchmark_Eval(b *testing.B) {
	// fEval, err := NewRuleset(dir)
	// if err != nil {
	// 	b.Fatal(err)
	// }
	ctx := context.Background()
	// dir := "D:/project/go/src/github.com/lxt1045/sigma_rule/run_236_new"

	bs, err := os.ReadFile("./test.json")
	if err != nil {
		b.Fatal(err)
	}
	x := Log{}
	rs, err := NewRulesetByDir(ctx, dir, x)
	if err != nil {
		b.Fatal(err)
	}

	var m map[string]interface{}
	dec := json.NewDecoder(bytes.NewBuffer([]byte(bs)))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		b.Fatal(err)
	}

	input, err := mapToStruct[Log](m)
	if err != nil {
		b.Fatal(err)
	}
	buf := make([]int64, 0, 16)
	ruleIDs := rs.Eval(&input, buf)
	b.Logf("ruleIDs:%+v", ruleIDs)

	for i := 0; i < 3; i++ {
		b.Run("test", func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				ruleIDs = rs.Eval(&input, buf)
			}
		})
	}
}

func showQps(ctx context.Context, msgSuss *int64) {
	ctx, _ = log.WithLogid(ctx, gid.GetGID())
	go func() {
		var lastCount int64
		lastTime := time.Now().UnixNano()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			last := atomic.LoadInt64(msgSuss)
			diff := last - lastCount
			t := time.Now().UnixNano()
			f := float64(diff) / (float64(t-lastTime) / float64(time.Second))
			log.Ctx(ctx).Warn().Float64("qps", f).Int64("count", last).Send()

			lastTime = t
			lastCount = last

			time.Sleep(time.Second * 2)
		}
	}()
}

func Test_ParseOffset(t *testing.T) {
	yml := &SigmaYml[Log]{
		Behavior: Behavior{
			Object:  "parent_username",
			Objects: []string{"parent_username", "++", "source"},
			Why:     "source",
			Subject: "platform",
		},
	}

	err := yml.ParseOffset()
	assert.Nil(t, err)
	t.Logf("%+v", yml.Offsets)
}

func Test_EqualFold(t *testing.T) {
	yes := strings.EqualFold("Linux", "linux")
	t.Logf("yes: %+v", yes)
}
