/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"gopkg.in/yaml.v2"
)

// PrintOutput renders a response body in the requested format.
func PrintOutput(w io.Writer, format string, body []byte) error {
	switch format {
	case "json":
		_, err := w.Write(body)
		return err
	case "yaml":
		var value interface{}
		if err := json.Unmarshal(body, &value); err != nil {
			return fmt.Errorf("unmarshal response as json: %w", err)
		}
		out, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		_, err = w.Write(out)
		return err
	case "table":
		return printTable(w, body)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func printTable(w io.Writer, body []byte) error {
	data, err := tableData(body)
	if err != nil {
		return err
	}
	switch value := data.(type) {
	case []interface{}:
		return printTableArray(w, value)
	case map[string]interface{}:
		return printTableObject(w, value)
	default:
		_, err := fmt.Fprintln(w, value)
		return err
	}
}

func tableData(body []byte) (interface{}, error) {
	var envelope struct {
		Data  interface{} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Error != nil {
			message := envelope.Error.Message
			if message == "" {
				message = "request returned an error"
			}
			return message, nil
		}
		return envelope.Data, nil
	}

	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, fmt.Errorf("unmarshal response as json: %w", err)
	}
	return value, nil
}

func printTableObject(w io.Writer, value map[string]interface{}) error {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, key := range keys {
		if _, err := fmt.Fprintf(tw, "%s\t%v\n", key, value[key]); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printTableArray(w io.Writer, rows []interface{}) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "(empty)")
		return err
	}

	first, ok := rows[0].(map[string]interface{})
	if !ok {
		for _, row := range rows {
			if _, err := fmt.Fprintln(w, row); err != nil {
				return err
			}
		}
		return nil
	}

	columns := make([]string, 0, len(first))
	for column := range first {
		columns = append(columns, column)
	}
	sort.Strings(columns)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, column := range columns {
		if _, err := fmt.Fprintf(tw, "%s\t", column); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(tw); err != nil {
		return err
	}
	for _, row := range rows {
		rowMap, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		for _, column := range columns {
			if _, err := fmt.Fprintf(tw, "%v\t", rowMap[column]); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(tw); err != nil {
			return err
		}
	}
	return tw.Flush()
}
