module github.com/kolosys/nova/examples/complete

go 1.24

replace github.com/kolosys/nova => ../..
replace github.com/kolosys/ion => ../../../ion

require (
	github.com/kolosys/ion v0.1.1
	github.com/kolosys/nova v0.0.0-00010101000000-000000000000
)
