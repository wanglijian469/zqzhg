package main

import (
	"encoding/json"
	"testing"
)

func TestAIIndependentInventoryAndEmptyResponse(t *testing.T) {
	index := map[int]AIPageIndex{
		1: {Blocks: []AIPageBlock{{BlockID: "p1-product", Title: "同型号旋耕机", Kind: "product"}, {BlockID: "p1-table", Title: "参数表", Kind: "support"}}},
		2: {Blocks: []AIPageBlock{{BlockID: "p2-product", Title: "同型号旋耕机", Kind: "product"}}},
		3: {Blocks: []AIPageBlock{{BlockID: "p3-table", Title: "共享尺寸图", Kind: "support"}}},
		4: {Blocks: []AIPageBlock{{BlockID: "p4-product", Title: "遗漏的结构产品", Kind: "product"}}},
	}
	in := AIInventory{Products: []AIInventoryProduct{{ProductName: "错误合并", BlockIDs: []string{"p1-product", "p2-product", "p3-table"}}}}
	out := aiPreserveIndependentProducts(index, in)
	if len(out.Products) != 3 {
		t.Fatal("independent products were lost", out)
	}
	if len(out.Products[0].BlockIDs) != 3 || len(out.Products[1].BlockIDs) != 2 {
		t.Fatal("same-page table or shared support missing", out)
	}
	for _, p := range out.Products {
		n := 0
		for _, id := range p.BlockIDs {
			if id == "p1-product" || id == "p2-product" || id == "p4-product" {
				n++
			}
		}
		if n != 1 {
			t.Fatal("separate primary blocks combined")
		}
	}
	var x AIProductExtraction
	if aiDecode(`{}`, &x) == nil {
		t.Fatal("empty response accepted")
	}
	if aiDecode(`{"products":[]}`, &x) == nil {
		t.Fatal("wrong wrapper accepted")
	}
	var idx AIPageIndex
	if aiDecode(`{}`, &idx) == nil {
		t.Fatal("empty page index accepted")
	}
	checkpoint := aiImportCheckpoint{Work: &AIImportWork{Product: "X", SourcePage: 2, Done: 3, Total: 12, Message: "复核"}}
	b, e := json.Marshal(AIImportTask{ID: 7, Checkpoint: aiJSON(checkpoint), Lease: "private-lease"})
	if e != nil {
		t.Fatal(e)
	}
	var task map[string]any
	json.Unmarshal(b, &task)
	if task["work"].(map[string]any)["done"] != float64(3) {
		t.Fatal("work progress not exposed")
	}
	if _, ok := task["checkpoint"]; ok {
		t.Fatal("private checkpoint exposed")
	}
	if _, ok := task["lease"]; ok {
		t.Fatal("private lease exposed")
	}
}
