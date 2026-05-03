package services

import (
	"archive/zip"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const PPTXMimeType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

type PowerPointSlide struct {
	Title        string
	Bullets      []string
	SpeakerNotes string
}

type PowerPointMaterialFile struct {
	OutputPath string
	FileName   string
	MimeType   string
}

func BuildPowerPointMaterialFile(presentationTitle, subtitle, subjectName, className string, slides []PowerPointSlide, outputDir string) (*PowerPointMaterialFile, error) {
	if strings.TrimSpace(presentationTitle) == "" {
		presentationTitle = "Materi Pembelajaran"
	}
	if len(slides) == 0 {
		return nil, fmt.Errorf("slides is required")
	}
	if strings.TrimSpace(outputDir) == "" {
		outputDir = "uploads"
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}

	fileName := fmt.Sprintf("%s-%d.pptx", sanitizePptxFileName(presentationTitle), time.Now().UnixMilli())
	outputPath := filepath.Join(outputDir, fileName)

	file, err := os.Create(outputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	slideTitles := make([]string, 0, len(slides))
	for _, slide := range slides {
		title := strings.TrimSpace(slide.Title)
		if title == "" {
			title = "Slide"
		}
		slideTitles = append(slideTitles, title)
	}

	if err := writeZipText(zw, "[Content_Types].xml", buildContentTypesXML(len(slides))); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "_rels/.rels", rootRelsXML()); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "docProps/app.xml", buildAppXML(slideTitles)); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "docProps/core.xml", buildCoreXML(presentationTitle, now)); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/presentation.xml", buildPresentationXML(len(slides))); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/_rels/presentation.xml.rels", buildPresentationRelsXML(len(slides))); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/presProps.xml", presPropsXML()); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/viewProps.xml", viewPropsXML()); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/tableStyles.xml", tableStylesXML()); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/theme/theme1.xml", themeXML()); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/slideMasters/slideMaster1.xml", slideMasterXML()); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/slideMasters/_rels/slideMaster1.xml.rels", slideMasterRelsXML()); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/slideLayouts/slideLayout1.xml", slideLayoutXML()); err != nil {
		return nil, err
	}
	if err := writeZipText(zw, "ppt/slideLayouts/_rels/slideLayout1.xml.rels", slideLayoutRelsXML()); err != nil {
		return nil, err
	}

	for index, slide := range slides {
		name := fmt.Sprintf("ppt/slides/slide%d.xml", index+1)
		content := buildSlideXML(slide, index, len(slides), presentationTitle, subjectName, className, subtitle)
		if err := writeZipText(zw, name, content); err != nil {
			return nil, err
		}
		relsName := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", index+1)
		if err := writeZipText(zw, relsName, slideRelsXML()); err != nil {
			return nil, err
		}
	}

	return &PowerPointMaterialFile{
		OutputPath: outputPath,
		FileName:   fileName,
		MimeType:   PPTXMimeType,
	}, nil
}

func sanitizePptxFileName(value string) string {
	sanitized := strings.ToLower(strings.TrimSpace(value))
	if sanitized == "" {
		return "materi-pembelajaran"
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range sanitized {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "materi-pembelajaran"
	}
	if len(result) > 80 {
		result = result[:80]
	}
	return result
}

func writeZipText(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(content))
	return err
}

func buildContentTypesXML(slideCount int) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	builder.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	builder.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/presProps.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presProps+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/viewProps.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.viewProps+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/tableStyles.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)
	builder.WriteString(`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	builder.WriteString(`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`)
	for i := 1; i <= slideCount; i++ {
		builder.WriteString(`<Override PartName="/ppt/slides/slide`)
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(`.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`)
	}
	builder.WriteString(`</Types>`)
	return builder.String()
}

func rootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
	<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
	<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`
}

func buildAppXML(slideTitles []string) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">`)
	builder.WriteString(`<TotalTime>0</TotalTime><Words>0</Words><Application>School System</Application><PresentationFormat>On-screen Show (16:9)</PresentationFormat><Paragraphs>0</Paragraphs><Slides>`)
	builder.WriteString(strconv.Itoa(maxInt(1, len(slideTitles))))
	builder.WriteString(`</Slides><Notes>0</Notes><HiddenSlides>0</HiddenSlides><MMClips>0</MMClips><ScaleCrop>false</ScaleCrop>`)
	builder.WriteString(`<HeadingPairs><vt:vector size="6" baseType="variant">`)
	builder.WriteString(`<vt:variant><vt:lpstr>Fonts Used</vt:lpstr></vt:variant><vt:variant><vt:i4>1</vt:i4></vt:variant>`)
	builder.WriteString(`<vt:variant><vt:lpstr>Theme</vt:lpstr></vt:variant><vt:variant><vt:i4>1</vt:i4></vt:variant>`)
	builder.WriteString(`<vt:variant><vt:lpstr>Slide Titles</vt:lpstr></vt:variant><vt:variant><vt:i4>`)
	builder.WriteString(strconv.Itoa(maxInt(1, len(slideTitles))))
	builder.WriteString(`</vt:i4></vt:variant>`)
	builder.WriteString(`</vt:vector></HeadingPairs>`)
	builder.WriteString(`<TitlesOfParts><vt:vector size="`)
	builder.WriteString(strconv.Itoa(maxInt(1, len(slideTitles)) + 2))
	builder.WriteString(`" baseType="lpstr">`)
	builder.WriteString(`<vt:lpstr>Arial</vt:lpstr><vt:lpstr>School System Theme</vt:lpstr>`)
	for _, title := range slideTitles {
		builder.WriteString(`<vt:lpstr>`)
		builder.WriteString(escapeXML(title))
		builder.WriteString(`</vt:lpstr>`)
	}
	builder.WriteString(`</vt:vector></TitlesOfParts>`)
	builder.WriteString(`<Company>School System</Company><LinksUpToDate>false</LinksUpToDate><SharedDoc>false</SharedDoc><HyperlinksChanged>false</HyperlinksChanged><AppVersion>16.0000</AppVersion>`)
	builder.WriteString(`</Properties>`)
	return builder.String()
}

func buildCoreXML(title, timestamp string) string {
	escaped := escapeXML(title)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
	<dc:title>%s</dc:title>
	<dc:subject>%s</dc:subject>
	<dc:creator>School System</dc:creator>
	<cp:lastModifiedBy>School System</cp:lastModifiedBy>
	<cp:revision>1</cp:revision>
	<dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>
	<dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>
</cp:coreProperties>`, escaped, escaped, timestamp, timestamp)
}

func buildPresentationXML(slideCount int) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" saveSubsetFonts="1" autoCompressPictures="0">`)
	builder.WriteString(`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>`)
	builder.WriteString(`<p:sldIdLst>`)
	for i := 1; i <= slideCount; i++ {
		builder.WriteString(`<p:sldId id="`)
		builder.WriteString(strconv.Itoa(255 + i))
		builder.WriteString(`" r:id="rId`)
		builder.WriteString(strconv.Itoa(1 + i))
		builder.WriteString(`"/>`)
	}
	builder.WriteString(`</p:sldIdLst>`)
	builder.WriteString(`<p:sldSz cx="12192000" cy="6858000"/><p:defaultTextStyle>`)
	builder.WriteString(`<a:lvl1pPr marL="0" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl1pPr>`)
	builder.WriteString(`<a:lvl2pPr marL="457200" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl2pPr>`)
	builder.WriteString(`<a:lvl3pPr marL="914400" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl3pPr>`)
	builder.WriteString(`</p:defaultTextStyle>`)
	builder.WriteString(`</p:presentation>`)
	return builder.String()
}

func buildPresentationRelsXML(slideCount int) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	builder.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for i := 1; i <= slideCount; i++ {
		builder.WriteString(`<Relationship Id="rId`)
		builder.WriteString(strconv.Itoa(1 + i))
		builder.WriteString(`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide`)
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(`.xml"/>`)
	}
	base := slideCount + 2
	builder.WriteString(`<Relationship Id="rId`)
	builder.WriteString(strconv.Itoa(base))
	builder.WriteString(`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps" Target="presProps.xml"/>`)
	builder.WriteString(`<Relationship Id="rId`)
	builder.WriteString(strconv.Itoa(base + 1))
	builder.WriteString(`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps" Target="viewProps.xml"/>`)
	builder.WriteString(`<Relationship Id="rId`)
	builder.WriteString(strconv.Itoa(base + 2))
	builder.WriteString(`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`)
	builder.WriteString(`<Relationship Id="rId`)
	builder.WriteString(strconv.Itoa(base + 3))
	builder.WriteString(`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles" Target="tableStyles.xml"/>`)
	builder.WriteString(`</Relationships>`)
	return builder.String()
}

func presPropsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:presentationPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`
}

func viewPropsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:viewPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:normalViewPr horzBarState="maximized"><p:restoredLeft sz="15611"/><p:restoredTop sz="94610"/></p:normalViewPr><p:slideViewPr><p:cSldViewPr snapToGrid="0" snapToObjects="1"><p:cViewPr varScale="1"><p:scale><a:sx n="136" d="100"/><a:sy n="136" d="100"/></p:scale><p:origin x="216" y="312"/></p:cViewPr><p:guideLst/></p:cSldViewPr></p:slideViewPr><p:notesTextViewPr><p:cViewPr><p:scale><a:sx n="1" d="1"/><a:sy n="1" d="1"/></p:scale><p:origin x="0" y="0"/></p:cViewPr></p:notesTextViewPr><p:gridSpacing cx="76200" cy="76200"/></p:viewPr>`
}

func tableStylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"/>`
}

func themeXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="School System Theme"><a:themeElements><a:clrScheme name="School System"><a:dk1><a:srgbClr val="111827"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="1F2937"/></a:dk2><a:lt2><a:srgbClr val="F3F4F6"/></a:lt2><a:accent1><a:srgbClr val="0EA5E9"/></a:accent1><a:accent2><a:srgbClr val="F97316"/></a:accent2><a:accent3><a:srgbClr val="22C55E"/></a:accent3><a:accent4><a:srgbClr val="A855F7"/></a:accent4><a:accent5><a:srgbClr val="EF4444"/></a:accent5><a:accent6><a:srgbClr val="14B8A6"/></a:accent6><a:hlink><a:srgbClr val="2563EB"/></a:hlink><a:folHlink><a:srgbClr val="7C3AED"/></a:folHlink></a:clrScheme><a:fontScheme name="School System"><a:majorFont><a:latin typeface="Aptos Display"/></a:majorFont><a:minorFont><a:latin typeface="Aptos"/></a:minorFont></a:fontScheme><a:fmtScheme name="School System"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`
}

func slideMasterXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst><p:hf sldNum="0" hdr="0" ftr="0" dt="0"/><p:txStyles><p:titleStyle><a:lvl1pPr algn="ctr" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:spcBef><a:spcPct val="0"/></a:spcBef><a:buNone/><a:defRPr sz="4400" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mj-lt"/><a:ea typeface="+mj-ea"/><a:cs typeface="+mj-cs"/></a:defRPr></a:lvl1pPr></p:titleStyle><p:bodyStyle><a:lvl1pPr marL="342900" indent="-342900" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:spcBef><a:spcPct val="20000"/></a:spcBef><a:buFont typeface="Arial" pitchFamily="34" charset="0"/><a:buChar char="•"/><a:defRPr sz="3200" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl1pPr></p:bodyStyle><p:otherStyle><a:defPPr><a:defRPr lang="en-US"/></a:defPPr><a:lvl1pPr marL="0" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl1pPr></p:otherStyle></p:txStyles></p:sldMaster>`
}

func slideMasterRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/></Relationships>`
}

func slideLayoutXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" preserve="1"><p:cSld name="DEFAULT"><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`
}

func slideLayoutRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`
}

func slideRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`
}

func buildSlideXML(slide PowerPointSlide, index, total int, presentationTitle, subjectName, className, subtitle string) string {
	topBar := []string{"0F172A", "0EA5E9"}
	barColor := topBar[index%len(topBar)]
	secondaryColor := "E2E8F0"
	title := escapeXML(strings.TrimSpace(slide.Title))
	if title == "" {
		title = fmt.Sprintf("Slide %d", index+1)
	}

	bullets := normalizeSlideBullets(slide.Bullets)
	notesText := strings.TrimSpace(slide.SpeakerNotes)

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`)
	builder.WriteString(`<p:cSld name="Slide `)
	builder.WriteString(strconv.Itoa(index + 1))
	builder.WriteString(`"><p:spTree>`)
	builder.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	builder.WriteString(`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	builder.WriteString(buildRectShape(2, "Top Bar", 0, 0, 12192000, 700000, barColor, "none", "none"))
	builder.WriteString(buildTextShape(3, "Presentation Title", 600000, 180000, 10400000, 280000, false, false, "FFFFFF", 24, "left", "mid", []string{presentationTitle}, false))
	builder.WriteString(buildTextShape(4, "Slide Title", 720000, 1080000, 10800000, 650000, false, true, "0F172A", 22, "left", "mid", []string{title}, false))
	builder.WriteString(buildTextShape(5, "Slide Body", 820000, 1820000, 9800000, 3600000, false, false, "1E293B", 18, "left", "top", bullets, true))

	if notesText != "" {
		builder.WriteString(buildTextShape(6, "Speaker Notes", 820000, 6070000, 10800000, 260000, false, true, "64748B", 10, "left", "top", []string{notesText}, false))
	}

	builder.WriteString(buildTextShape(7, "Footer", 700000, 6500000, 8000000, 160000, false, false, "64748B", 10, "left", "mid", []string{subtitleText(subjectName, className, subtitle)}, false))
	builder.WriteString(buildTextShape(8, "Slide Number", 11150000, 6500000, 500000, 160000, false, true, "0F172A", 10, "right", "mid", []string{fmt.Sprintf("%d/%d", index+1, total)}, false))
	builder.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="9" name="Accent"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="9300000" y="1820000"/><a:ext cx="2600000" cy="3600000"/></a:xfrm><a:prstGeom prst="roundRect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="`)
	builder.WriteString(secondaryColor)
	builder.WriteString(`"/></a:solidFill><a:ln><a:solidFill><a:srgbClr val="`)
	builder.WriteString(barColor)
	builder.WriteString(`"/></a:solidFill></a:ln></p:spPr><p:txBody><a:bodyPr wrap="square" rtlCol="0" anchor="ctr"/><a:lstStyle/><a:p><a:pPr><a:buNone/></a:pPr><a:r><a:rPr lang="id-ID" dirty="0"><a:solidFill><a:srgbClr val="`)
	builder.WriteString(barColor)
	builder.WriteString(`"/></a:solidFill></a:rPr><a:t>`)
	builder.WriteString(escapeXML(subjectName))
	builder.WriteString(`</a:t></a:r><a:endParaRPr lang="id-ID" dirty="0"/></a:p><a:p><a:pPr><a:buNone/></a:pPr><a:r><a:rPr lang="id-ID" dirty="0"><a:solidFill><a:srgbClr val="`)
	builder.WriteString(barColor)
	builder.WriteString(`"/></a:solidFill></a:rPr><a:t>`)
	builder.WriteString(escapeXML(className))
	builder.WriteString(`</a:t></a:r><a:endParaRPr lang="id-ID" dirty="0"/></a:p></p:txBody></p:sp>`)
	builder.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)
	return builder.String()
}

func buildRectShape(id int, name string, x, y, w, h int, fillColor, lineColor, lineWidth string) string {
	var builder strings.Builder
	builder.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="`)
	builder.WriteString(strconv.Itoa(id))
	builder.WriteString(`" name="`)
	builder.WriteString(escapeXML(name))
	builder.WriteString(`"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="`)
	builder.WriteString(strconv.Itoa(x))
	builder.WriteString(`" y="`)
	builder.WriteString(strconv.Itoa(y))
	builder.WriteString(`"/><a:ext cx="`)
	builder.WriteString(strconv.Itoa(w))
	builder.WriteString(`" cy="`)
	builder.WriteString(strconv.Itoa(h))
	builder.WriteString(`"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="`)
	builder.WriteString(fillColor)
	builder.WriteString(`"/></a:solidFill>`)
	if lineColor == "none" {
		builder.WriteString(`<a:ln><a:noFill/></a:ln>`)
	} else {
		builder.WriteString(`<a:ln`)
		if strings.TrimSpace(lineWidth) != "" {
			builder.WriteString(` w="`)
			builder.WriteString(strings.TrimSpace(lineWidth))
			builder.WriteString(`"`)
		}
		builder.WriteString(`><a:solidFill><a:srgbClr val="`)
		builder.WriteString(lineColor)
		builder.WriteString(`"/></a:solidFill></a:ln>`)
	}
	builder.WriteString(`</p:spPr></p:sp>`)
	return builder.String()
}

func buildTextShape(id int, name string, x, y, w, h int, bold, italic bool, color string, fontSize int, align, valign string, lines []string, bulleted bool) string {
	var builder strings.Builder
	builder.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="`)
	builder.WriteString(strconv.Itoa(id))
	builder.WriteString(`" name="`)
	builder.WriteString(escapeXML(name))
	builder.WriteString(`"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="`)
	builder.WriteString(strconv.Itoa(x))
	builder.WriteString(`" y="`)
	builder.WriteString(strconv.Itoa(y))
	builder.WriteString(`"/><a:ext cx="`)
	builder.WriteString(strconv.Itoa(w))
	builder.WriteString(`" cy="`)
	builder.WriteString(strconv.Itoa(h))
	builder.WriteString(`"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr><p:txBody><a:bodyPr wrap="square" rtlCol="0"`)
	if strings.EqualFold(valign, "mid") {
		builder.WriteString(` anchor="ctr"`)
	} else if strings.EqualFold(valign, "bottom") {
		builder.WriteString(` anchor="b"`)
	} else {
		builder.WriteString(` anchor="t"`)
	}
	builder.WriteString(`><a:spAutoFit/></a:bodyPr><a:lstStyle/>`)

	if len(lines) == 0 {
		lines = []string{""}
	}
	if bulleted {
		builder.WriteString(buildBulletParagraphs(lines, color, fontSize))
	} else {
		for _, line := range lines {
			builder.WriteString(`<a:p><a:pPr`)
			if strings.EqualFold(align, "center") {
				builder.WriteString(` algn="ctr"`)
			} else if strings.EqualFold(align, "right") {
				builder.WriteString(` algn="r"`)
			} else {
				builder.WriteString(` algn="l"`)
			}
			builder.WriteString(`><a:buNone/></a:pPr><a:r><a:rPr lang="id-ID" dirty="0" sz="`)
			builder.WriteString(strconv.Itoa(fontSize * 100))
			builder.WriteString(`"`)
			if bold {
				builder.WriteString(` b="1"`)
			}
			if italic {
				builder.WriteString(` i="1"`)
			}
			builder.WriteString(`><a:solidFill><a:srgbClr val="`)
			builder.WriteString(color)
			builder.WriteString(`"/></a:solidFill></a:rPr><a:t>`)
			builder.WriteString(escapeXML(line))
			builder.WriteString(`</a:t></a:r><a:endParaRPr lang="id-ID" dirty="0"/></a:p>`)
		}
	}
	builder.WriteString(`</p:txBody></p:sp>`)
	return builder.String()
}

func buildBulletParagraphs(lines []string, color string, fontSize int) string {
	if len(lines) == 0 {
		return `<a:p><a:pPr><a:buNone/></a:pPr><a:r><a:rPr lang="id-ID" dirty="0" sz="1800"><a:solidFill><a:srgbClr val="64748B"/></a:solidFill></a:rPr><a:t></a:t></a:r><a:endParaRPr lang="id-ID" dirty="0"/></a:p>`
	}

	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(`<a:p><a:pPr marL="342900" indent="-228600"><a:buChar char="•"/></a:pPr><a:r><a:rPr lang="id-ID" dirty="0" sz="`)
		builder.WriteString(strconv.Itoa(fontSize * 100))
		builder.WriteString(`"><a:solidFill><a:srgbClr val="`)
		builder.WriteString(color)
		builder.WriteString(`"/></a:solidFill></a:rPr><a:t>`)
		builder.WriteString(escapeXML(line))
		builder.WriteString(`</a:t></a:r><a:endParaRPr lang="id-ID" dirty="0"/></a:p>`)
	}
	return builder.String()
}

func normalizeSlideBullets(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		out = append(out, item)
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func subtitleText(subjectName, className, subtitle string) string {
	if strings.TrimSpace(subtitle) != "" {
		return strings.TrimSpace(subtitle)
	}

	parts := make([]string, 0, 3)
	if strings.TrimSpace(subjectName) != "" {
		parts = append(parts, strings.TrimSpace(subjectName))
	}
	if strings.TrimSpace(className) != "" {
		parts = append(parts, strings.TrimSpace(className))
	}
	if strings.TrimSpace(subtitle) != "" {
		parts = append(parts, strings.TrimSpace(subtitle))
	}
	return strings.Join(parts, " • ")
}

func escapeXML(value string) string {
	return html.EscapeString(strings.TrimSpace(value))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
