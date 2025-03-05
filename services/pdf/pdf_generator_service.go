package pdfgeneratorservice

import (
	"hirevo/internal/handlers"
	"strings"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// PDFData represents the PDF data
type PDFData struct {
	Title       string
	HeaderImage []byte
	Header      string
	Content     map[string]string
	Footer      string
}

// GeneratePDFBytes   generate PDF and returns []byte.
func GeneratePDFBytes(info PDFData) ([]byte, error) {
	m, err := generatePDF(info)
	document, err := m.Generate()
	if err != nil {
		handlers.LogError(err, "Failed Maroto generate PDF")
		return nil, err
	}

	pdfBytes := document.GetBytes()
	return pdfBytes, nil
}

func generatePDF(data PDFData) (core.Maroto, error) {
	cfg := config.NewBuilder().
		WithPageNumber().
		WithLeftMargin(10).
		WithTopMargin(15).
		WithRightMargin(10).
		Build()

	mrt := maroto.New(cfg)
	m := maroto.NewMetricsDecorator(mrt)

	if err := m.RegisterHeader(getPageHeader(data.HeaderImage, data.Header)); err != nil {
		handlers.LogError(err, "Failed RegisterHeader generate PDF")
		return nil, err
	}
	if err := m.RegisterFooter(getPageFooter(data.Footer)); err != nil {
		handlers.LogError(err, "Failed RegisterFooter generate PDF")
		return nil, err
	}

	// Title
	m.AddRow(7,
		text.NewCol(3, data.Title, props.Text{
			Top:   1.5,
			Size:  9,
			Left:  2,
			Style: fontstyle.Bold,
			Align: align.Left,
			Color: &props.WhiteColor,
		}),
	).WithStyle(&props.Cell{BackgroundColor: getDarkGrayColor()})

	// Content/Metadata
	contentRows := getPageContent(data.Content)
	m.AddRows(contentRows...)

	return m, nil
}

func getPageHeader(logoBytes []byte, content string) core.Row {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	textComponents := buildTextComponents(lines)

	var height float64
	if len(lines) < 4 {
		height = 36
	} else {
		height = float64(len(lines) * 8)
	}

	headerRow := row.New(height)

	if len(logoBytes) > 0 {
		ext := extension.Type("png")
		headerRow.Add(
			image.NewFromBytesCol(3, logoBytes, ext, props.Rect{
				Center:  false,
				Percent: 80,
			}),
			col.New(3),
		)
	} else {
		headerRow.Add(
			col.New(6),
		)
	}

	headerRow.Add(
		col.New(6).Add(textComponents...),
	)

	return headerRow
}

func getPageContent(values map[string]string) []core.Row {
	var rows []core.Row
	for k, v := range values {
		r := row.New(5).Add(

			text.NewCol(3, k, props.Text{
				Top:   4,
				Style: fontstyle.Bold,
				Size:  8,
				Left:  2,
				Align: align.Left,
			}),
			text.NewCol(8, v, props.Text{
				Top:   4,
				Size:  8,
				Align: align.Left,
			}),
		)
		rows = append(rows, r)
	}
	return rows
}

func getPageFooter(content string) core.Row {
	return row.New(40).Add(
		col.New(6).Add(
			text.New(content, props.Text{
				Size:  8,
				Align: align.Right,
				Style: fontstyle.Normal,
				Color: getDarkGrayColor(),
			}),
		),
	)
}

// Helpers
func buildTextComponents(lines []string) []core.Component {
	var comps []core.Component
	for i, line := range lines {
		comps = append(comps, text.New(line, props.Text{
			Top:   float64(4 * i),
			Size:  8,
			Align: align.Right,
			Style: fontstyle.Normal,
			Color: getDarkGrayColor(),
		}))
	}
	return comps
}
func getDarkGrayColor() *props.Color {
	return &props.Color{
		Red:   55,
		Green: 55,
		Blue:  55,
	}
}
