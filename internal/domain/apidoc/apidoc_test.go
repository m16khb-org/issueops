package apidoc

import (
	"reflect"
	"testing"

	contract "issueops/internal/contract/apidoc"
)

func codes(vs []contract.Violation) []string {
	out := []string{}
	for _, v := range vs {
		out = append(out, v.Code)
	}
	return out
}

func TestIsCandidate(t *testing.T) {
	cases := map[string]bool{
		"src/users.controller.ts": true,
		"src/UserDTO.ts":          true,
		"api/openapi.yaml":        true,
		"docs/api.md":             false,
		"notes/api.txt":           false,
		"package.json":            false,
		"api/yarn.lock":           false,
		"src/util.ts":             false,
	}
	for f, want := range cases {
		if got := IsCandidate(f); got != want {
			t.Errorf("IsCandidate(%q)=%v want %v", f, got, want)
		}
	}
}

func TestValidReviewVerdict(t *testing.T) {
	for v, want := range map[string]bool{"pass": true, "fail": true, "Pass": false, "": false} {
		if ValidReviewVerdict(v) != want {
			t.Errorf("verdict %q want %v", v, want)
		}
	}
}

func TestStaticKinds(t *testing.T) {
	cases := []struct {
		file     string
		ctl, dto bool
	}{
		{"a.controller.ts", true, false},
		{"A.Router.ts", true, false},
		{"a.dto.ts", false, true},
		{"a.schema.ts", false, true},
		{"a.controller.js", false, false},
		{"plain.ts", false, false},
	}
	for _, c := range cases {
		ctl, dto := StaticKinds(c.file)
		if ctl != c.ctl || dto != c.dto {
			t.Errorf("%s: got %v,%v", c.file, ctl, dto)
		}
	}
}

func TestSortViolations(t *testing.T) {
	vs := []contract.Violation{
		{File: "b", Line: 1, Code: "x"},
		{File: "a", Line: 2, Code: "z"},
		{File: "a", Line: 2, Code: "y"},
		{File: "a", Line: 1, Code: "q"},
	}
	SortViolations(vs)
	var got []string
	for _, v := range vs {
		got = append(got, v.File+string(rune('0'+v.Line))+v.Code)
	}
	want := []string{"a1q", "a2y", "a2z", "b1x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

const goodOp = "@ApiOperation({ summary: 's', description: '### A' })\n"

func TestCheckNestController(t *testing.T) {
	cases := []struct {
		name, text string
		want       []string
	}{
		{"fully documented", goodOp + "@Get('users')\nasync list() {}", []string{}},
		{"no operation", "@Get('users')\nasync list() {}", []string{"missing_api_operation"}},
		{"operation without description", "@ApiOperation({ summary: 's' })\n@Get()\nasync list() {}", []string{"missing_api_operation_description"}},
		{"description not sectioned", "@ApiOperation({ summary: 's', description: 'plain' })\n@Get()\nasync list() {}", []string{"invalid_api_operation_description_format"}},
		{"not a route", "async helper() {}", []string{}},
		{"path param undocumented", goodOp + "@Get('users/:id')\nasync one() {}", []string{"missing_api_param"}},
		{"path param documented", goodOp + "@ApiParam({ name: 'id' })\n@Get('users/:id')\nasync one() {}", []string{}},
		{"headers undocumented", goodOp + "@Get()\nasync h(@Headers() h) {}", []string{"missing_api_header", "missing_400_response"}},
		{"named query undocumented", goodOp + "@ApiBadRequestResponse()\n@Get()\nasync q(@Query('page') p) {}", []string{"missing_api_query"}},
		{"named body undocumented", goodOp + "@ApiBadRequestResponse()\n@Post()\nasync b(@Body('x') x) {}", []string{"missing_api_body"}},
		{"body with 400 by status name", goodOp + "@ApiResponse({ status: HttpStatus.BAD_REQUEST })\n@Post()\nasync b(@Body() x) {}", []string{}},
		{"private route lacks 401", goodOp + "@UseGuards(G)\n@Get()\nasync p() {}", []string{"missing_401_response"}},
		{"private route with 401", goodOp + "@UseGuards(G)\n@ApiResponses([{ status: 401 }])\n@Get()\nasync p() {}", []string{}},
		{"public overrides class guard", "@UseGuards(G)\nclass C {\n" + goodOp + "@Public()\n@Get()\nasync p() {}\n}", []string{}},
		{"class guard makes private", "@UseGuards(G)\nclass C {\n" + goodOp + "@Get()\nasync p() {}\n}", []string{"missing_401_response"}},
		{"multiline operation decorator", "@ApiOperation({\n summary: 's',\n description: '### A',\n})\n@Get()\nasync m() {}", []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vs := CheckNestController("f.ts", c.text)
			if got := codes(vs); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
			for _, v := range vs {
				if v.File != "f.ts" || v.Line < 1 {
					t.Fatalf("bad location %+v", v)
				}
			}
		})
	}
}

func TestHasNestResponseStatus(t *testing.T) {
	cases := []struct {
		name, block string
		status      int
		want        bool
	}{
		{"shorthand 400", "@ApiBadRequestResponse()", 400, true},
		{"shorthand 401", "@ApiUnauthorizedResponse()", 401, true},
		{"shorthand wrong status", "@ApiBadRequestResponse()", 401, false},
		{"numeric", "@ApiResponse({ status: 404 })", 404, true},
		{"named", "@ApiResponse({ status: HttpStatus.NOT_FOUND })", 404, true},
		{"array form", "@ApiResponses([{ status: 200 }, { status: 409 }])", 409, true},
		{"other status", "@ApiResponse({ status: 200 })", 404, false},
		{"unknown name", "@ApiResponse({ status: HttpStatus.WEIRD })", 400, false},
		{"none", "@Get()", 400, false},
	}
	for _, c := range cases {
		if got := HasNestResponseStatus(c.block, c.status); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestCheckNestDTO(t *testing.T) {
	wrap := func(body string) string { return "export class D {\n" + body + "\n}\n" }
	cases := []struct {
		name, text string
		want       []string
	}{
		{"required documented", wrap("@ApiProperty()\n@IsString()\nname: string;"), []string{}},
		{"required undocumented", wrap("@IsString()\nname: string;"), []string{"missing_api_property"}},
		{"required with optional swagger", wrap("@ApiPropertyOptional()\nname: string;"), []string{"required_optional_mismatch", "missing_api_property"}},
		{"required flag false", wrap("@ApiProperty({ required: false })\nname: string;"), []string{"required_optional_mismatch"}},
		{"optional documented", wrap("@ApiPropertyOptional()\n@IsOptional()\nname?: string;"), []string{}},
		{"optional missing everything", wrap("name?: string;"), []string{"missing_api_property_optional", "missing_is_optional"}},
		{"optional but documented required", wrap("@ApiProperty()\n@IsOptional()\nname?: string;"), []string{"required_optional_mismatch"}},
		{"optional with required false", wrap("@ApiProperty({ required: false })\n@IsOptional()\nname?: string;"), []string{}},
		{"method ignored", wrap("foo(): void {}"), []string{}},
		{"static ignored", wrap("static x: number;"), []string{}},
		{"no class", "const a: number = 1;", []string{}},
		{"decorators reset after method", wrap("@ApiProperty()\nfoo() {}\nname: string;"), []string{"missing_api_property"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := codes(CheckNestDTO("d.ts", c.text)); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
