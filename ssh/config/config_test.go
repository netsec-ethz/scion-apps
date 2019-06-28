package config

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConfig(t *testing.T) {
	Convey("Given a config struct with string option A and []string option B", t, func() {
		myStruct := &(struct {
			A string   `regex:"(abc|\\d*)"`
			B []string `regex:"[xyzesno]*"`
		}{
			A: "1337",
			B: []string{"meme"},
		})

		Convey("They should have their default values", func() {
			So(myStruct.A, ShouldEqual, "1337")
			So(myStruct.B[0], ShouldEqual, "meme")
		})

		Convey("We should be able to set legal values", func() {
			err := Set(myStruct, "A", "abc")
			So(err, ShouldEqual, nil)
			So(myStruct.A, ShouldEqual, "abc")

			err = UpdateFromString(myStruct, "A 000000001000")
			So(err, ShouldEqual, nil)
			So(myStruct.A, ShouldEqual, "000000001000")

			b, err := SetIfNot(myStruct, "B", true, false)
			So(err, ShouldEqual, nil)
			So(b, ShouldEqual, false)
			So(len(myStruct.B), ShouldEqual, 2)
			So(myStruct.B[1], ShouldEqual, "yes")

			b, err = SetIfNot(myStruct, "A", 8, 8)
			So(err, ShouldEqual, nil)
			So(b, ShouldEqual, true)
			So(myStruct.A, ShouldEqual, "000000001000")
		})

		Convey("We should not be able to set illegal values", func() {
			err := Set(myStruct, "A", "abcd")
			So(err, ShouldNotEqual, nil)
			So(myStruct.A, ShouldEqual, "1337")

			err = UpdateFromString(myStruct, "A abc def")
			So(err, ShouldNotEqual, nil)
			So(myStruct.A, ShouldEqual, "1337")

			b, err := SetIfNot(myStruct, "B", 17, 18)
			So(err, ShouldNotEqual, nil)
			So(b, ShouldEqual, false)
			So(len(myStruct.B), ShouldEqual, 1)
		})
	})
}
