package deepseek

import (
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/tidwall/gjson"
)

type responsesToolCounter struct {
	counts map[string]int
	seen   map[string]bool
	final  bool
}

func (counter *responsesToolCounter) Observe(raw []byte) map[string]int {
	output := gjson.GetBytes(raw, "output")
	if !output.Exists() {
		output = gjson.GetBytes(raw, "response.output")
	}
	if output.IsArray() && len(output.Array()) > 0 {
		counter.final = true
		counter.counts = map[string]int{SearchToolPriceKey: 0, FetchToolPriceKey: 0}
		for _, item := range output.Array() {
			if item.Get("type").String() != dto.BuildInCallWebSearchCall || item.Get("status").String() != "completed" {
				continue
			}
			if item.Get("action.type").String() == "open_page" {
				counter.counts[FetchToolPriceKey]++
			} else {
				counter.counts[SearchToolPriceKey]++
			}
		}
		return counter.counts
	}
	if counter.final || gjson.GetBytes(raw, "type").String() != dto.ResponsesOutputTypeItemDone {
		return nil
	}
	item := gjson.GetBytes(raw, "item")
	if item.Get("type").String() != dto.BuildInCallWebSearchCall || item.Get("status").String() != "completed" {
		return nil
	}
	identity := item.Get("id").String()
	if identity == "" {
		if index := gjson.GetBytes(raw, "output_index"); index.Exists() {
			identity = "index:" + index.Raw
		} else {
			identity = item.Raw
		}
	}
	if counter.seen[identity] {
		return nil
	}
	if counter.seen == nil {
		counter.seen = make(map[string]bool)
		counter.counts = map[string]int{SearchToolPriceKey: 0, FetchToolPriceKey: 0}
	}
	counter.seen[identity] = true
	key := SearchToolPriceKey
	if item.Get("action.type").String() == "open_page" {
		key = FetchToolPriceKey
	}
	counter.counts[key]++
	return counter.counts
}
