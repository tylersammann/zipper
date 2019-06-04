package zippermerge

import (
	"github.com/hhrutter/pdfcpu/pkg/log"
	pdf "github.com/hhrutter/pdfcpu/pkg/pdfcpu"
)

func patchIndRef(ir *pdf.IndirectRef, lookup map[int]int) {
	i := ir.ObjectNumber.Value()
	ir.ObjectNumber = pdf.Integer(lookup[i])
}

func patchObject(o pdf.Object, lookup map[int]int) pdf.Object {

	log.Trace.Printf("patchObject before: %v\n", o)

	var ob pdf.Object

	switch obj := o.(type) {

	case pdf.IndirectRef:
		patchIndRef(&obj, lookup)
		ob = obj

	case pdf.Dict:
		patchDict(obj, lookup)
		ob = obj

	case pdf.StreamDict:
		patchDict(obj.Dict, lookup)
		ob = obj

	case pdf.ObjectStreamDict:
		patchDict(obj.Dict, lookup)
		ob = obj

	case pdf.XRefStreamDict:
		patchDict(obj.Dict, lookup)
		ob = obj

	case pdf.Array:
		patchArray(obj, lookup)
		ob = obj

	}

	log.Trace.Printf("patchObject end: %v\n", ob)

	return ob
}

func patchDict(d pdf.Dict, lookup map[int]int) {

	log.Trace.Printf("patchDict before: %v\n", d)

	for k, obj := range d {
		o := patchObject(obj, lookup)
		if o != nil {
			d[k] = o
		}
	}

	log.Trace.Printf("patchDict after: %v\n", d)
}

func patchArray(a pdf.Array, lookup map[int]int) {

	log.Trace.Printf("patchArray begin: %v\n", a)

	for i, obj := range a {
		o := patchObject(obj, lookup)
		if o != nil {
			a[i] = o
		}
	}

	log.Trace.Printf("patchArray end: %v\n", a)
}

func objNrsIntSet(ctx *pdf.Context) pdf.IntSet {

	objNrs := pdf.IntSet{}

	for k := range ctx.Table {
		if k == 0 {
			// obj#0 is always the head of the freelist.
			continue
		}
		objNrs[k] = true
	}

	return objNrs
}

func lookupTable(keys pdf.IntSet, i int) map[int]int {

	m := map[int]int{}

	for k := range keys {
		m[k] = i
		i++
	}

	return m
}

// Patch an IntSet of objNrs using lookup.
func patchObjects(s pdf.IntSet, lookup map[int]int) pdf.IntSet {

	t := pdf.IntSet{}

	for k, v := range s {
		if v {
			t[lookup[k]] = v
		}
	}

	return t
}

func patchSourceObjectNumbers(ctxSource, ctxDest *pdf.Context) {

	log.Debug.Printf("patchSourceObjectNumbers: ctxSource: xRefTableSize:%d trailer.Size:%d - %s\n", len(ctxSource.Table), *ctxSource.Size, ctxSource.Read.FileName)
	log.Debug.Printf("patchSourceObjectNumbers:   ctxDest: xRefTableSize:%d trailer.Size:%d - %s\n", len(ctxDest.Table), *ctxDest.Size, ctxDest.Read.FileName)

	// Patch source xref tables obj numbers which are essentially the keys.
	//logInfoMerge.Printf("Source XRefTable before:\n%s\n", ctxSource)

	objNrs := objNrsIntSet(ctxSource)

	// Create lookup table for object numbers.
	// The first number is the successor of the last number in ctxDest.
	lookup := lookupTable(objNrs, *ctxDest.Size)

	// Patch pointer to root object
	patchIndRef(ctxSource.Root, lookup)

	// Patch pointer to info object
	if ctxSource.Info != nil {
		patchIndRef(ctxSource.Info, lookup)
	}

	// Patch free object zero
	entry := ctxSource.Table[0]
	off := int(*entry.Offset)
	if off != 0 {
		i := int64(lookup[off])
		entry.Offset = &i
	}

	// Patch all indRefs for xref table entries.
	for k := range objNrs {

		//logDebugMerge.Printf("patching obj #%d\n", k)

		entry := ctxSource.Table[k]

		if entry.Free {
			log.Debug.Printf("patch free entry: old offset:%d\n", *entry.Offset)
			off := int(*entry.Offset)
			if off == 0 {
				continue
			}
			i := int64(lookup[off])
			entry.Offset = &i
			log.Debug.Printf("patch free entry: new offset:%d\n", *entry.Offset)
			continue
		}

		patchObject(entry.Object, lookup)
	}

	// Patch xref entry object numbers.
	m := make(map[int]*pdf.XRefTableEntry, *ctxSource.Size)
	for k, v := range lookup {
		m[v] = ctxSource.Table[k]
	}
	m[0] = ctxSource.Table[0]
	ctxSource.Table = m

	// Patch DuplicateInfo object numbers.
	ctxSource.Optimize.DuplicateInfoObjects = patchObjects(ctxSource.Optimize.DuplicateInfoObjects, lookup)

	// Patch Linearization object numbers.
	ctxSource.LinearizationObjs = patchObjects(ctxSource.LinearizationObjs, lookup)

	// Patch XRefStream objects numbers.
	ctxSource.Read.XRefStreams = patchObjects(ctxSource.Read.XRefStreams, lookup)

	// Patch object stream object numbers.
	ctxSource.Read.ObjectStreams = patchObjects(ctxSource.Read.ObjectStreams, lookup)

	log.Debug.Printf("patchSourceObjectNumbers end")
}

func ZipperMergePageTrees(ctx2, ctx1 *pdf.Context, rev2, rev1 bool) error {

	log.Debug.Println("ZipperMergePageTrees begin")

	indRefPageTreeRootDict1, err := ctx1.Pages()
	if err != nil {
		return err
	}
	pageTreeRootDict1, _ := ctx1.XRefTable.DereferenceDict(*indRefPageTreeRootDict1)

	indRefPageTreeRootDict2, err := ctx2.Pages()
	if err != nil {
		return err
	}
	pageTreeRootDict2, _ := ctx2.XRefTable.DereferenceDict(*indRefPageTreeRootDict2)

	pages1 := pageTreeRootDict1.ArrayEntry("Kids")
	pages2 := pageTreeRootDict2.ArrayEntry("Kids")

	//pageTreeRootDict2.Insert("Parent", *indRefPageTreeRootDict1)

	// pop and then append mergedPages page from ctx1 and then ctx2 until the arrays are empty
	// if one array runs out first, choose pages from the other until it is empty
	mergedPages := pdf.Array{}
	for len(pages1) > 0 || len(pages2) > 0 {
		mergedPages, pages1 = appendNextPage(mergedPages, pages1, rev1)
		mergedPages, pages2 = appendNextPage(mergedPages, pages2, rev2)
	}

	pageTreeRootDict1.Update("Count", pdf.Integer(len(mergedPages)))
	pageTreeRootDict1.Update("Kids", mergedPages)
	ctx1.PageCount = len(mergedPages)

	log.Debug.Println("ZipperMergePageTrees end")

	return nil
}

func appendNextPage(dest, src pdf.Array, reverse bool) (pdf.Array, pdf.Array) {
	if len(src) == 0 {
		return dest, src
	}
	if reverse {
		dest = append(dest, src[len(src)-1])
		src = src[:len(src)-1]
		return dest, src
	}
	dest = append(dest, src[0])
	src = src[1:]
	return dest, src
}

func appendSourceObjectsToDest(ctxSource, ctxDest *pdf.Context) {

	log.Debug.Println("appendSourceObjectsToDest begin")

	for objNr, entry := range ctxSource.Table {

		// Do not copy free list head.
		if objNr == 0 {
			continue
		}

		log.Debug.Printf("adding obj %d from src to dest\n", objNr)

		ctxDest.Table[objNr] = entry

		*ctxDest.Size++

	}

	log.Debug.Println("appendSourceObjectsToDest end")
}

// merge two disjunct IntSets
func mergeIntSets(src, dest pdf.IntSet) {
	for k := range src {
		dest[k] = true
	}
}

func mergeDuplicateObjNumberIntSets(ctxSource, ctxDest *pdf.Context) {

	log.Debug.Println("mergeDuplicateObjNumberIntSets begin")

	mergeIntSets(ctxSource.Optimize.DuplicateInfoObjects, ctxDest.Optimize.DuplicateInfoObjects)
	mergeIntSets(ctxSource.LinearizationObjs, ctxDest.LinearizationObjs)
	mergeIntSets(ctxSource.Read.XRefStreams, ctxDest.Read.XRefStreams)
	mergeIntSets(ctxSource.Read.ObjectStreams, ctxDest.Read.ObjectStreams)

	log.Debug.Println("mergeDuplicateObjNumberIntSets end")
}

// ZipperMergeXRefTables merges Context ctxSource into ctxDest by appending its page tree.
func ZipperMergeXRefTables(ctxSource, ctxDest *pdf.Context, revSource, revDest bool) (err error) {

	// Sweep over ctxSource cross ref table and ensure valid object numbers in ctxDest's space.
	patchSourceObjectNumbers(ctxSource, ctxDest)

	// Append ctxSource pageTree to ctxDest pageTree.
	log.Debug.Println("ZipperMergePageTrees")
	err = ZipperMergePageTrees(ctxSource, ctxDest, revSource, revDest)
	if err != nil {
		return err
	}

	// Append ctxSource objects to ctxDest
	log.Debug.Println("appendSourceObjectsToDest")
	appendSourceObjectsToDest(ctxSource, ctxDest)

	// Mark source's root object as free.
	err = ctxDest.DeleteObject(int(ctxSource.Root.ObjectNumber))
	if err != nil {
		return
	}

	// Mark source's info object as free.
	// Note: Any indRefs this info object depends on are missed.
	if ctxSource.Info != nil {
		err = ctxDest.DeleteObject(int(ctxSource.Info.ObjectNumber))
		if err != nil {
			return
		}
	}

	// Merge all IntSets containing redundant object numbers.
	log.Debug.Println("mergeDuplicateObjNumberIntSets")
	mergeDuplicateObjNumberIntSets(ctxSource, ctxDest)

	log.Info.Printf("Dest XRefTable after merge:\n%s\n", ctxDest)

	return nil
}
