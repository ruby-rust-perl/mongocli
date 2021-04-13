// Copyright 2021 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package rest was mostly inspired by github.com/spf13/cobra/doc
// but with some changes to match the expected formats and styles of our writers and tools.
package rest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// GenTree generates the docs for the full tree of commands
func GenTree(cmd *cobra.Command, dir string) error {
	emptyStr := func(s string) string { return "" }
	return GenTreeCustom(cmd, dir, emptyStr, defaultLinkHandler)
}

// GenTreeCustom is the the same as GenTree, but
// with custom filePrepender and linkHandler.
func GenTreeCustom(cmd *cobra.Command, dir string, filePrepender func(string) string, linkHandler func(string, string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := GenTreeCustom(c, dir, filePrepender, linkHandler); err != nil {
			return err
		}
	}

	basename := strings.ReplaceAll(cmd.CommandPath(), " ", "-") + ".txt"
	filename := filepath.Join(dir, basename)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.WriteString(f, filePrepender(filename)); err != nil {
		return err
	}
	if err := GenCustom(cmd, f, linkHandler); err != nil {
		return err
	}
	return nil
}

// GenCustom creates custom reStructured Text output.
// Adapted from github.com/spf13/cobra/doc to match MongoDB tooling and style
func GenCustom(cmd *cobra.Command, w io.Writer, linkHandler func(string, string) string) error {
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	buf := new(bytes.Buffer)
	name := cmd.CommandPath()

	short := cmd.Short
	long := cmd.Long
	if long == "" {
		long = short
	}
	ref := strings.ReplaceAll(name, " ", "_")

	buf.WriteString(".. _" + ref + ":\n\n")
	buf.WriteString(strings.Repeat("=", len(name)) + "\n")
	buf.WriteString(name + "\n")
	buf.WriteString(strings.Repeat("=", len(name)) + "\n\n")
	buf.WriteString(short + "\n")
	buf.WriteString("\n" + long + "\n\n")

	if cmd.Runnable() {
		buf.WriteString(fmt.Sprintf(".. code-block::\n\n   %s\n\n", strings.ReplaceAll(cmd.UseLine(), "[flags]", "[options]")))
	}

	if err := printOptionsReST(buf, cmd); err != nil {
		return err
	}

	if len(cmd.Example) > 0 {
		buf.WriteString("Examples\n")
		buf.WriteString("~~~~~~~~\n\n")
		buf.WriteString(fmt.Sprintf(".. code-block::\n%s\n\n", indentString(cmd.Example, " ")))
	}

	if hasSeeAlso(cmd) {
		buf.WriteString("See Also\n")
		buf.WriteString("~~~~~~~~\n\n")
		if cmd.HasParent() {
			parent := cmd.Parent()
			pname := parent.CommandPath()
			ref = strings.ReplaceAll(pname, " ", "_")
			buf.WriteString(fmt.Sprintf("* %s \t - %s\n", linkHandler(pname, ref), parent.Short))
			cmd.VisitParents(func(c *cobra.Command) {
				if c.DisableAutoGenTag {
					cmd.DisableAutoGenTag = c.DisableAutoGenTag
				}
			})
		}

		children := cmd.Commands()
		sort.Sort(byName(children))

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}
			cname := name + " " + child.Name()
			ref = strings.ReplaceAll(cname, " ", "_")
			buf.WriteString(fmt.Sprintf("* %s \t - %s\n", linkHandler(cname, ref), child.Short))
		}
		buf.WriteString("\n")
	}
	if !cmd.DisableAutoGenTag {
		buf.WriteString("*Auto generated by MongoDB CLI on " + time.Now().Format("2-Jan-2006") + "*\n")
	}
	_, err := buf.WriteTo(w)
	return err
}

// Test to see if we have a reason to print See Also information in docs
// Basically this is a test for a parent command or a subcommand which is
// both not deprecated and not the autogenerated help command.
func hasSeeAlso(cmd *cobra.Command) bool {
	if cmd.HasParent() {
		return true
	}
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		return true
	}
	return false
}

type byName []*cobra.Command

func (s byName) Len() int           { return len(s) }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }

func printOptionsReST(buf *bytes.Buffer, cmd *cobra.Command) error {
	flags := cmd.NonInheritedFlags()
	if flags.HasAvailableFlags() {
		buf.WriteString("Options\n")
		buf.WriteString("~~~~~~~\n\n.. list-table::\n")
		buf.WriteString(`   :header-rows: 1
   :widths: 20 10 60 10

   * - Option
     - Type
     - Description
     - Required
`)
		buf.WriteString(indentString(FlagUsages(flags), " "))
		buf.WriteString("\n")
	}

	parentFlags := cmd.InheritedFlags()
	if parentFlags.HasAvailableFlags() {
		buf.WriteString("Inherited Options\n")
		buf.WriteString("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~\n\n.. list-table::\n")
		buf.WriteString(`   :header-rows: 1
   :widths: 20 10 60 10

   * - Option
     - Type
     - Description
     - Required
`)
		buf.WriteString(indentString(FlagUsages(parentFlags), " "))
		buf.WriteString("\n")
	}
	return nil
}

// linkHandler for default ReST hyperlink markup
func defaultLinkHandler(name, ref string) string {
	return fmt.Sprintf("`%s <%s.txt>`_", name, ref)
}

// adapted from: https://github.com/kr/text/blob/main/indent.go
func indentString(s, p string) string {
	var res []byte
	b := []byte(s)
	prefix := []byte(p)
	bol := true
	for _, c := range b {
		if bol && c != '\n' {
			res = append(res, prefix...)
		}
		res = append(res, c)
		bol = c == '\n'
	}
	return string(res)
}
