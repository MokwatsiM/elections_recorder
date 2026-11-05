// Command exportgeo writes the recorded IEC boundary files as GeoJSON with only
// the join keys, ready for tippecanoe.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"lge-dashboard/backend/internal/iec"
	"lge-dashboard/backend/internal/store"
)

type feature struct {
	Type       string          `json:"type"`
	Properties map[string]any  `json:"properties"`
	Geometry   json.RawMessage `json:"geometry"`
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: exportgeo <output dir>")
	}
	out := os.Args[1]
	ctx := context.Background()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://lge:lge@localhost:5432/lge?sslmode=disable"
	}
	st, err := store.Open(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	const base = "ElectionMap/MapData/2026/"

	read := func(path string) []feature {
		body, err := st.Latest(ctx, base+path)
		if err != nil {
			log.Fatalf("%s: %v (has the seed finished?)", path, err)
		}
		var fc struct{ Features []feature }
		if err := json.Unmarshal(body, &fc); err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		return fc.Features
	}
	keep := func(fs []feature, from, to string) []feature {
		for i := range fs {
			fs[i].Type = "Feature"
			fs[i].Properties = map[string]any{to: fs[i].Properties[from]}
		}
		return fs
	}
	write := func(name string, fs []feature) {
		f, err := os.Create(filepath.Join(out, name+".geojson"))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := json.NewEncoder(f).Encode(map[string]any{"type": "FeatureCollection", "features": fs}); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %d features\n", name, len(fs))
	}

	write("provinces", keep(read("nationalProvince.js"), "ProvinceId", "id"))
	write("municipalities", keep(read("nationalMunic.js"), "MunicId", "id"))
	var wards, vds []feature
	for _, m := range iec.Municipalities() {
		wards = append(wards, keep(read(fmt.Sprintf("municward%d.js", m.ID)), "PklWardID", "id")...)
		vds = append(vds, keep(read(fmt.Sprintf("municvd%d.js", m.ID)), "VDNumber", "id")...)
	}
	write("wards", wards)
	write("vds", vds)
}
