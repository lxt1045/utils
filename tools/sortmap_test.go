package tools

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"unsafe"
)

func TestCtx(t *testing.T) {

	type CtxKey1 struct{}
	type CtxValue1 struct {
		V1 string
		V2 int
	}

	ctx := context.TODO()
	ctx = context.WithValue(ctx, CtxKey1{}, &CtxValue1{
		V1: "v1",
		V2: 12222,
	})
	t.Logf("1:%x", unsafe.Pointer(ctx.Value(CtxKey1{}).(*CtxValue1)))
	t.Logf("1:%+v", *ctx.Value(CtxKey1{}).(*CtxValue1))

	ctx = context.WithValue(ctx, CtxKey1{}, &CtxValue1{
		V1: "v2",
		V2: 12222,
	})
	t.Logf("2:%x", unsafe.Pointer(ctx.Value(CtxKey1{}).(*CtxValue1)))
	t.Logf("2:%+v", *ctx.Value(CtxKey1{}).(*CtxValue1))
}
func TestMap(t *testing.T) {
	m := map[string]interface{}{
		"1": 11,
		"2": 22,
		"3": map[string]interface{}{
			"4": 44,
			"5": 55,
		},
	}
	mm := NewMap(m, nil, false, true)
	bs, err := json.MarshalIndent(&mm, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bs))
}
func TestMap2(t *testing.T) {
	m := map[string]interface{}{
		"1": 11,
		"2": 22,
		"3": map[string]interface{}{
			"4": 44,
			"5": 55,
		},
	}
	mm := NewMap(m, []string{"1", "3"}, false, true)
	bs, err := json.MarshalIndent(&mm, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bs))

	bs, err = json.MarshalIndent(&mm, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bs))
}

func TestMap1(t *testing.T) {
	str := `
	
{
  "hardware_id": "d9ac5c54-ecf4-4b1f-a09e-cbc5205d7909",
  "is_hardware_id_real": false,
  "anon_id": "f90eff0d-75a9-4901-a57c-8eeb587b4f4e",
  "brand": "Xiaomi",
  "model": "MI 6",
  "screen_dpi": 480,
  "screen_height": 1920,
  "screen_width": 1080,
  "wifi": true,
  "ui_mode": "UI_MODE_TYPE_NORMAL",
  "os": "Android",
  "os_version": 34,
  "country": "US",
  "language": "en",
  "local_ip": "172.30.16.5",
  "cpu_type": "aarch64",
  "build": "AP2A.240905.003 test-keys",
  "locale": "en_US",
  "connection_type": "wifi",
  "os_version_android": "14",
  "debug": false,
  "partner_data": {},
  "app_version": "8.41.0a",
  "update": 0,
  "latest_install_time": 1741145854357,
  "latest_update_time": 1741145854357,
  "first_install_time": 1741145854357,
  "previous_update_time": 0,
  "environment": "FULL_APP",
  "UDID": "3ded5a76-5180-e0fd-f98a-7351eb937154",
  "metadata": {
    "UDID": "3ded5a76-5180-e0fd-f98a-7351eb937154",
    "$google_analytics_client_id": "3ded5a76-5180-e0fd-f98a-7351eb937154"
  },
  "branch_sdk_request_timestamp": 1741146008705,
  "branch_sdk_request_unique_id": "d383560d-63f1-421b-a30e-608edd128b34-2025030503",
  "install_referrer_extras": "utm_source=google-play&utm_medium=organic",
  "app_store": "PlayStore",
  "advertising_ids": {
    "aaid": "f4cee763-378b-4916-aa5a-10ee98718925"
  },
  "lat_val": 0,
  "google_advertising_id": "f4cee763-378b-4916-aa5a-10ee98718925",
  "sdk": "android5.15.1",
  "branch_key": "key_live_hdB4TO81954vePrg11rLmkjpyyd7wMc5",
  "retryNumber": 0
}
`
	m := map[string]interface{}{}
	err := json.Unmarshal([]byte(str), &m)
	if err != nil {
		t.Fatal(err)
	}

	mm := NewMap(m, nil, false, true)
	bs, err := json.MarshalIndent(&mm, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bs))
}

func TestFmt(t *testing.T) {
	ff := 999.555
	t.Log(fmt.Sprintf("%.3f", ff))
	t.Log(strconv.FormatFloat(ff, 'f', 3, 64))
}

func TestPW(t *testing.T) {
	Md5 := func(s string) string {
		h := md5.New()
		h.Write([]byte(s))
		return hex.EncodeToString(h.Sum(nil))
	}
	Md5Password := func(password string) string {
		salt := "etcd-manage"
		return Md5(Md5(password) + Md5(salt))
	}
	t.Logf("pw:%s", Md5Password("1234"))
}

func TestFmtMap(t *testing.T) {
	str1 := `{
    "payload": "{\"e\":\"{\\\"responseCode\\\":\\\"0\\\",\\\"signedData\\\":\\\"0|-640160840|com.sisal.sisalfunclub|593|ANlOHQNTeg2NTzxpeHtl7xYFZTKR9EnBOg==|1748483951242:VT=9223372036854775807\\\",\\\"signature\\\":\\\"fc54rvFZ3eoFE5jcPJVHCS0AWqgjSI0ipVrFBcfhhBxp679QHaTjKDt01Mhz8U\\\\\\/L43wp\\\\\\/8mXesGEynI5Z+ptPy9hHWnzkVMt4wkR3ertsZOo10ul+8NA7RT0ZAdxO6zBFgqO7LjEqIBPy\\\\\\/tTWcNiQp2D2RnOMyHH9mEarBarcoRJRbjfnc\\\\\\/vdUaDvyvwdxy0zy9JvrWe6s8hp8Z5RQvyt1R4oOjZ1KGHHUDimBdXvLerbTz\\\\\\/wwlAhF1HdheeA8zlmxOwmA1ogIpshMiuWGf5HvFjmefSgfjoEKh8uZlq8mlapmcUaj6HWkZjmlNCOa20AxSVXrZfTqkMIbCkHYvI7w==\\\",\\\"is_revenue_event\\\":false}\"}",
    "signature": "6521c4b47889a31a4010fc5f85616f93c6263ceb"
}
	`
	_ = str1

	f := func(str string) {
		m1, m2, m3 := map[string]interface{}{}, map[string]interface{}{}, map[string]interface{}{}
		err := json.Unmarshal([]byte(str), &m1)
		if err != nil {
			t.Fatal(err)
		}
		payload := m1["payload"].(string)
		err = json.Unmarshal([]byte(payload), &m2)
		if err != nil {
			t.Fatal(err)
		}
		e := m2["e"].(string)
		err = json.Unmarshal([]byte(e), &m3)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("%+v", m3)

		bs, _ := json.Marshal(NewMap(m3, []string{"responseCode", "signedData", "signature", "is_revenue_event"}, true, false))
		t.Logf("%s", bs)
		m2["e"] = string(bs)
		bs, _ = json.Marshal(NewMap(m2, []string{"responseCode", "signedData", "signature", "is_revenue_event"}, true, false))
		t.Logf("%s", bs)
		m1["payload"] = string(bs)
		bs, _ = json.Marshal(NewMap(m1, []string{"responseCode", "signedData", "signature", "is_revenue_event"}, true, false))
		t.Logf("%s", bs)
	}
	_ = f

	// f(str1)
	// f(str2)

	{
		m1, m2, m3 := map[string]interface{}{}, map[string]interface{}{}, map[string]interface{}{}
		m3 = map[string]interface{}{
			"is_revenue_event": false,
			"responseCode":     0,
			"signature":        `fc54rvFZ3eoFE5jcPJVHCS0AWqgjSI0ipVrFBcfhhBxp679QHaTjKDt01Mhz8U\/L43wp\/8mXesGEynI5Z+ptPy9hHWnzkVMt4wkR3ertsZOo10ul+8NA7RT0ZAdxO6zBFgqO7LjEqIBPy\/tTWcNiQp2D2RnOMyHH9mEarBarcoRJRbjfnc\/vdUaDvyvwdxy0zy9JvrWe6s8hp8Z5RQvyt1R4oOjZ1KGHHUDimBdXvLerbTz\/wwlAhF1HdheeA8zlmxOwmA1ogIpshMiuWGf5HvFjmefSgfjoEKh8uZlq8mlapmcUaj6HWkZjmlNCOa20AxSVXrZfTqkMIbCkHYvI7w==`,
			"signedData":       "0|-640160840|com.sisal.sisalfunclub|593|ANlOHQNTeg2NTzxpeHtl7xYFZTKR9EnBOg==|1748483951242:VT=9223372036854775807",
		}

		t.Logf("%+v", m3)

		// bs, _ := json.Marshal(NewMap(m3, []string{"responseCode", "signedData", "signature", "is_revenue_event"}, true, false))
		bs, _ := json.Marshal(m3)
		t.Logf("%s", bs)
		m2["e"] = string(bs)
		// bs, _ = json.Marshal(NewMap(m2, []string{"responseCode", "signedData", "signature", "is_revenue_event"}, true, false))
		bs, _ = json.Marshal(m2)
		t.Logf("%s", bs)
		m1["payload"] = string(bs)
		// bs, _ = json.Marshal(NewMap(m1, []string{"responseCode", "signedData", "signature", "is_revenue_event"}, true, false))
		bs, _ = json.Marshal(m1)
		t.Logf("%s", bs)
	}
}
