package sidecar

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

var _ = Describe("Test sidecar apphelper", func() {
	It("should find the single gtid set in backup", func() {
		var (
			source string = "mysql-bin.000002        6552870 684ca0cf-495e-11e9-9fe8-0a580af407e9:1-176661\n"
			result string = "684ca0cf-495e-11e9-9fe8-0a580af407e9:1-176661"
		)
		Expect(getGTIDFrom(strings.NewReader(source))).To(Equal(result))
	})
	It("should find all gtid sets in backup", func() {
		var (
			source string = `mysql-bin.006394      154349713       00003306-1111-0000-0000-000000000001:1-48861335,
00003306-1111-1111-1111-111111111111:1-11000155952,
00003306-2222-2222-2222-222222222222:1-8706021957
`
			result string = "00003306-1111-0000-0000-000000000001:1-48861335," +
				"00003306-1111-1111-1111-111111111111:1-11000155952," +
				"00003306-2222-2222-2222-222222222222:1-8706021957"
		)
		Expect(getGTIDFrom(strings.NewReader(source))).To(Equal(result))
	})

})
