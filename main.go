package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/calmh/ipfix"
	"github.com/honeycombio/libhoney-go"
)

var ipfixhoneyVersion string

type Config struct {
	WriteKey string
	Dataset  string
}

type InterpretedRecord struct {
	ExportTime uint32               `json:"exportTime"`
	TemplateId uint16               `json:"templateId"`
	Fields     []myInterpretedField `json:"fields"`
}

// Because we want to control JSON serialization
type myInterpretedField struct {
	Name         string      `json:"name"`
	EnterpriseId uint32      `json:"enterprise,omitempty"`
	FieldId      uint16      `json:"field"`
	Value        interface{} `json:"value,omitempty"`
	RawValue     []int       `json:"raw,omitempty"`
}

func integers(s []byte) []int {
	if s == nil {
		return nil
	}

	r := make([]int, len(s))
	for i := range s {
		r[i] = int(s[i])
	}
	return r
}

func timestamps(s []byte) int64 {
	if s == nil {
		return 0
	}
	const longForm = "2016-12-28T20:51:41-0500"
	t, err := time.Parse(longForm, string(s))
	if err == nil {
		fmt.Println(t)
		return t.Unix()
	}
	return 0
}

// Reads info from config file
func ReadConfig(configfile string) Config {
	_, err := os.Stat(configfile)
	if err != nil {
		log.Fatal("Config file is missing: ", configfile)
	}

	var config Config
	if _, err := toml.DecodeFile(configfile, &config); err != nil {
		log.Fatal(err)
	}
	return config
}

func messagesGenerator(r io.Reader, s *ipfix.Session, i *ipfix.Interpreter) <-chan []InterpretedRecord {
	c := make(chan []InterpretedRecord)

	errors := 0
	go func() {
		for {
			msg, err := s.ParseReader(r)
			if err == io.EOF {
				close(c)
				return
			}
			if err != nil {
				errors++
				if errors > 3 {
					panic(err)
				} else {
					log.Println(err)
				}
				continue
			} else {
				errors = 0
			}

			irecs := make([]InterpretedRecord, len(msg.DataRecords))
			for j, record := range msg.DataRecords {
				ifs := i.Interpret(record)
				mfs := make([]myInterpretedField, len(ifs))
				for k, iif := range ifs {
					mfs[k] = myInterpretedField{iif.Name, iif.EnterpriseID, iif.FieldID, iif.Value, integers(iif.RawValue)}
				}
				ir := InterpretedRecord{msg.Header.ExportTime, record.TemplateID, mfs}
				irecs[j] = ir
			}

			c <- irecs
		}
	}()

	return c
}

func main() {
	log.Println("ipfix-honeycomb", ipfixhoneyVersion)
	// Set Up Honeycomb.io
	var config = ReadConfig("app.conf")
	// call Init before using libhoney
	libhoney.Init(libhoney.Config{
		WriteKey:   config.WriteKey,
		Dataset:    config.Dataset,
		SampleRate: 1,
	})
	// when all done, call close
	defer libhoney.Close()

	s := ipfix.NewSession()
	i := ipfix.NewInterpreter(s)
	msgs := messagesGenerator(os.Stdin, s, i)
	// outputencoder := json.NewEncoder(os.Stdout)
	for {
		select {
		case irecs, ok := <-msgs:
			if !ok {
				return
			}
			for _, rec := range irecs {
				for i := range rec.Fields {
					f := &rec.Fields[i]
					switch v := f.Value.(type) {
					case []byte:
						f.RawValue = integers(v)
						f.Value = nil
					}
				}
				// Write to StandardOut
				// outputencoder.Encode(rec)
				// Send to Honeycomb.io
				ev := libhoney.NewEvent()
				fmt.Println(rec)
				ev.AddField("ExportTime", rec.ExportTime)
				for myfieldindex := range rec.Fields {
					ev.AddField(rec.Fields[myfieldindex].Name, rec.Fields[myfieldindex].Value)
				}
				err := ev.Send()
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}
}
