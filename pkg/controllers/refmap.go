// Copyright 2019 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"k8s.io/apimachinery/pkg/types"
)

type refMap struct {
	srcDsts map[types.NamespacedName]map[string]bool // src -> dsts
	dstSrcs map[types.NamespacedName]map[string]bool // dst -> srcs
}

func (m *refMap) set(namespace, src string, dsts map[string]bool) {
	for dst := range m.dsts(namespace, src) {
		if !dsts[dst] {
			m.rem(namespace, src, dst)
		}
	}
	for dst := range dsts {
		if !m.has(namespace, src, dst) {
			m.add(namespace, src, dst)
		}
	}
}

func (m *refMap) add(namespace, src, dst string) {
	srcName := types.NamespacedName{Namespace: namespace, Name: src}
	dsts, ok := m.srcDsts[srcName]
	if !ok {
		if m.srcDsts == nil {
			m.srcDsts = make(map[types.NamespacedName]map[string]bool)
		}
		dsts = make(map[string]bool)
		m.srcDsts[srcName] = dsts
	}
	dsts[dst] = true

	dstName := types.NamespacedName{Namespace: namespace, Name: dst}
	srcs, ok := m.dstSrcs[dstName]
	if !ok {
		if m.dstSrcs == nil {
			m.dstSrcs = make(map[types.NamespacedName]map[string]bool)
		}
		srcs = make(map[string]bool)
		m.dstSrcs[dstName] = srcs
	}
	srcs[src] = true
}

func (m *refMap) rem(namespace, src, dst string) {
	srcName := types.NamespacedName{Namespace: namespace, Name: src}
	if delete(m.srcDsts[srcName], dst); len(m.srcDsts[srcName]) == 0 {
		delete(m.srcDsts, srcName)
	}

	dstName := types.NamespacedName{Namespace: namespace, Name: dst}
	if delete(m.dstSrcs[dstName], src); len(m.dstSrcs[dstName]) == 0 {
		delete(m.dstSrcs, dstName)
	}
}

func (m *refMap) dsts(namespace, src string) map[string]bool {
	return m.srcDsts[types.NamespacedName{Namespace: namespace, Name: src}]
}

func (m *refMap) srcs(namespace, dst string) map[string]bool {
	return m.dstSrcs[types.NamespacedName{Namespace: namespace, Name: dst}]
}

func (m *refMap) has(namespace, src, dst string) bool {
	return m.dsts(namespace, src)[dst]
}
