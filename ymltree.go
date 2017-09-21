/*
	Nested YAML Configuration Builder
*/
package ymltree

import (
	"errors"
	"fmt"
	//"github.com/davecgh/go-spew/spew"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strings"
)

//capture everything inside ${...}
const ENV_REPLACER_REGEX = "\\$\\{([^\\}]+)\\}"
const ENV_REPLACER_PREFIX = "${"
const ENV_REPLACER_SUFFIX = "}"

type ConfigMap interface {
	Export() []byte
	Find(searchPath string) (node interface{})
	FindDefault(searchPath string, def string) (out string)
	FindDefaultInt(searchPath string, def int) (out int)
	Select(path string) (ret Map, err error)
	Templatize(data Map)
	Dump()
}

//store yaml data here
type Map map[interface{}]interface{}

//LoadRaw YAML data into ConfigMap
func LoadRaw(data []byte) (cmap Map, err error) {
	err = yaml.Unmarshal([]byte(data), &cmap)
	return
}

//Load YAML config from file
func Load(fn string) (ret Map, err error) {

	data, err := ioutil.ReadFile(fn)
	if err == nil && len(data) > 0 {
		ret, err = LoadRaw(data)
	}

	return
}

//Export ConfigMap back to YAML
func (me Map) Export() []byte {
	out, _ := yaml.Marshal(me)
	return out
}

func templateReplacer(v string, env Map, reg *regexp.Regexp) string {
	matches := reg.FindAllString(v, -1)
	for _, match := range matches {

		envKey := strings.Replace(
			strings.Replace(match, ENV_REPLACER_SUFFIX, "", 1), ENV_REPLACER_PREFIX, "", 1)

		//any environment coming from the OS has precedent over what's configured
		newVal := os.Getenv(envKey)
		if newVal == "" && env[envKey] != nil {
			newVal = env[envKey].(string)
		}
		v = strings.Replace(v, match, newVal, -1)
	}

	return v
}

//Templatize a configurator.Map object which replaces "${...}" with the value from env[...]
func (me Map) Templatize(env Map) {
	reg := regexp.MustCompile(ENV_REPLACER_REGEX)
	for k, v := range me {
		if reflect.TypeOf(v).Kind() == reflect.String {
			me[k] = templateReplacer(v.(string), env, reg)
		} else if reflect.TypeOf(v).Kind() == reflect.Map {
			v.(ConfigMap).Templatize(env)
		} else if reflect.TypeOf(v).Kind() == reflect.Slice {
			for idx, item := range v.([]interface{}) {
				if reflect.TypeOf(item).Kind() == reflect.String {
					v.([]interface{})[idx] = templateReplacer(item.(string), env, reg)
				} else if reflect.TypeOf(item).Kind() == reflect.Map {
					//not tested...
					item.(Map).Templatize(env)
				}
			}

		}
	}
}

func (me Map) Dump() {

	fmt.Printf("%s", me.Export())
	//spew.Dump(me.Export())
}

func (me Map) FindDefaultInt(searchPath string, def int) (out int) {

	ret := me.Find(searchPath)

	if ret != nil && reflect.TypeOf(ret).Kind() == reflect.Int {
		return ret.(int)
	}

	return def

}

func (me Map) FindDefault(searchPath string, def string) (out string) {

	ret := me.Find(searchPath)
	if ret == nil {
		return def
	}

	return ret.(string)
}

//Find element specified by searchPath in nested Map
//Example: result := mymap.Find('/elem1/subelem2/elem3')
func (me Map) Find(searchPath string) (node interface{}) {
	pathArr := strings.Split(searchPath, "/")
	node = me[pathArr[0]]
	if node != nil {
		for _, path := range pathArr[1:] {
			if reflect.Map == reflect.TypeOf(node).Kind() {
				node, _ = node.(Map)[path]
			} else {
				node = nil
				break
			}
		}
	}
	return
}

func addToEnv(m Map) {
	for k, v := range m {
		if os.Getenv(k.(string)) == "" {
			os.Setenv(k.(string), v.(string))
		}
	}
}

//Select and expand the YAML configuration's nested files with environment overrides
func (me Map) Select(path string) (selmap Map, err error) {

	parentEnv := me.Find("_env")
	if parentEnv != nil {
		addToEnv(parentEnv.(Map))
	}

	elem := me.Find(path)
	if elem == nil {
		err = errors.New(fmt.Sprintf("Can't find the path '%s' in this map", path))
		return
	}

	extends := elem.(Map).getParentList()
	for i := 0; i < len(extends); i++ {

		extender := extends[i]

		if parentEnv != nil && reflect.TypeOf(parentEnv).Kind() == reflect.Map {
			extender.(Map).Templatize(parentEnv.(Map))
		} else if parentEnv == nil {
			extender.(Map).Templatize(nil)
		}

		file := extender.(Map)["file"]
		service := extender.(Map)["service"]
		path := extender.(Map)["path"]

		if file == nil || len(file.(string)) == 0 {
			err = errors.New("YAML Nested configuration contains errors: 'file' key is unset")
			return
		}

		fi, _ := Load(file.(string))

		var es interface{}
		if service != nil && len(service.(string)) > 0 {
			es = fi.Find(service.(string))
		} else {
			es = fi
		}

		if es == nil {
			err = errors.New(fmt.Sprintf("Can't find service '%s' in nested config '%s'", service, file))
			return
		}

		e := es.(Map).getParentList()
		if e != nil {
			for _, v := range e {
				extends = append(extends, v)
			}
		}

		var env map[interface{}]interface{}

		localEnv := fi.Find("_env")
		if localEnv != nil {
			addToEnv(localEnv.(Map))
		}

		if parentEnv != nil && reflect.TypeOf(parentEnv).Kind() == reflect.Map {
			if localEnv == nil {
				env = parentEnv.(Map)
			} else {
				env = localEnv.(Map)
				for k, v := range parentEnv.(Map) {
					env[k] = v
				}

			}
		} else if localEnv != nil {
			env = localEnv.(Map)
		}

		var elempath Map
		if path == nil {
			elempath = elem.(Map)
		} else {
			elem.(Map)[path] = make(Map)
			elempath = elem.(Map)[path].(Map)
		}

		eservice := es.(Map)
		eservice.Templatize(env)
		for k, v := range eservice {
			if k != "extends" {
				elempath.Merge(false, eservice, k, v)
			}
		}

		elem.(Map).Templatize(env)
	}

	selmap = elem.(Map)

	return
}

func (dstMap Map) Merge(override bool, srcMap interface{}, key interface{}, val interface{}) {

	if _, ok := dstMap[key]; !ok {
		dstMap[key] = val
	} else {
		valType := reflect.TypeOf(val).Kind()

		switch valType {
		case reflect.Map:
			nm := srcMap.(Map)[key]
			if "map" == reflect.TypeOf(nm).Kind().String() {
				for k, v := range nm.(Map) {
					if reflect.TypeOf(dstMap[key]).Kind() == reflect.Map {
						dstMap[key].(Map).Merge(override, srcMap.(Map)[key], k, v)
					} else {
						panic(fmt.Sprintf("unexpected type %s in Map.merge", reflect.TypeOf(key).Kind().String()))
					}
				}
			}
			break
		case reflect.Slice:
			//merge slices
			if reflect.TypeOf(dstMap[key]).Kind() == reflect.Slice {
				pval := dstMap[key].([]interface{})
				for _, v := range val.([]interface{}) {
					skip := false
					for _, iv := range pval {
						if iv == v {
							skip = true
						}
					}
					if !skip {
						pval = append(pval, v)
					}
				}
				dstMap[key] = pval
			} else {
				dstMap[key] = val
			}

			break
		case reflect.String:
			if override && dstMap[key] != val {
				dstMap[key] = val
			}
		}

	}

}

func (me Map) getParentList() (extends []interface{}) {

	e := me.Find("extends")
	if e != nil {
		if reflect.TypeOf(e).Kind() == reflect.Slice {
			extends = e.([]interface{})
		} else if reflect.TypeOf(e).Kind() == reflect.Map {
			extends = append(extends, e.(Map))
		}
	}
	return

}
