// Copied from https://cs.opensource.google/go/x/exp/+/24438e51023af3bfc1db8aed43c1342817e8cfcd:rand/regress_test.go

// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test that random number sequences generated by a specific seed
// do not change from version to version.
//
// If the generator changes, the golden outputs need updating, and
// client programs may break. Although the desire for compatibility
// is not as stringent as in the original math/rand package,
// when possible avoid changing the generator.

package rand_test

import (
	"flag"
	"fmt"
	"reflect"
	"testing"

	. "github.com/hongyuyang/mongo-go-driver/internal/rand"
)

var printgolden = flag.Bool("printgolden", false, "print golden results for regression test")

// TestSource verifies that the output of the default Source is locked down.
func TestSourceRegress(t *testing.T) {
	src := NewSource(1)
	var got [20]uint64
	for i := range got {
		got[i] = src.Uint64()
	}
	want := [20]uint64{
		0x34e936394905d167,
		0x817c0ef62fe4c731,
		0x937987e6e24f5a40,
		0x0c0a8307fe226199,
		0xf96363568d8bab56,
		0xbaef3af36bd02620,
		0x8f18e416eb6b936b,
		0x05a43fc149f3a67a,
		0xdab012eb3ce01697,
		0xf76c495a133c6aa9,
		0x304b24c5040ce457,
		0x47d77e0abb413159,
		0x52a810fa9e452f04,
		0x2d24b66380cf4780,
		0x5ec7691b92018ef5,
		0x5076dfa749261ea0,
		0xac8f11ad3941d213,
		0x13fa8d67de91db25,
		0xb50883a9893274eb,
		0xeb8f59263f9109ac,
	}
	if got != want {
		t.Errorf("got:\n\t%#016x\nwant:\n\t%#016x", got, want)
		if *printgolden {
			for _, x := range got {
				fmt.Printf("\t\t%#016x,\n", x)
			}
		}
	}
}

// TestRegress validates that the output stream is locked down, for instance so
// optimizations do not change the output. It iterates over methods of the
// Rand type to find functions to evaluate and checks the first 20 results
// against the golden results.
func TestRegress(t *testing.T) {
	var int32s = []int32{1, 10, 32, 1 << 20, 1<<20 + 1, 1000000000, 1 << 30, 1<<31 - 2, 1<<31 - 1}
	var int64s = []int64{1, 10, 32, 1 << 20, 1<<20 + 1, 1000000000, 1 << 30, 1<<31 - 2, 1<<31 - 1, 1000000000000000000, 1 << 60, 1<<63 - 2, 1<<63 - 1}
	var uint64s = []uint64{1, 10, 32, 1 << 20, 1<<20 + 1, 1000000000, 1 << 30, 1<<31 - 2, 1<<31 - 1, 1000000000000000000, 1 << 60, 1<<64 - 2, 1<<64 - 1}
	var permSizes = []int{0, 1, 5, 8, 9, 10, 16}
	var readBufferSizes = []int{1, 7, 8, 9, 10}
	r := New(NewSource(0))

	rv := reflect.ValueOf(r)
	n := rv.NumMethod()
	p := 0
	if *printgolden {
		fmt.Printf("var regressGolden = []interface{}{\n")
	}
	for i := 0; i < n; i++ {
		m := rv.Type().Method(i)
		mv := rv.Method(i)
		mt := mv.Type()
		if mt.NumOut() == 0 {
			continue
		}
		r.Seed(0)
		if *printgolden && i > 0 {
			fmt.Println()
		}
		for repeat := 0; repeat < 20; repeat++ {
			var args []reflect.Value
			var argstr string
			if mt.NumIn() == 1 {
				var x interface{}
				switch mt.In(0).Kind() {
				default:
					t.Fatalf("unexpected argument type for r.%s", m.Name)

				case reflect.Int:
					if m.Name == "Perm" {
						x = permSizes[repeat%len(permSizes)]
						break
					}
					big := int64s[repeat%len(int64s)]
					if int64(int(big)) != big {
						r.Int63n(big) // what would happen on 64-bit machine, to keep stream in sync
						if *printgolden {
							fmt.Printf("\tskipped, // must run printgolden on 64-bit machine\n")
						}
						p++
						continue
					}
					x = int(big)

				case reflect.Int32:
					x = int32s[repeat%len(int32s)]

				case reflect.Int64:
					x = int64s[repeat%len(int64s)]

				case reflect.Uint64:
					x = uint64s[repeat%len(uint64s)]

				case reflect.Slice:
					if m.Name == "Read" {
						n := readBufferSizes[repeat%len(readBufferSizes)]
						x = make([]byte, n)
					}
				}
				argstr = fmt.Sprint(x)
				args = append(args, reflect.ValueOf(x))
			}

			var out interface{}
			out = mv.Call(args)[0].Interface()
			if m.Name == "Int" || m.Name == "Intn" {
				out = int64(out.(int))
			}
			if m.Name == "Read" {
				out = args[0].Interface().([]byte)
			}
			if *printgolden {
				var val string
				big := int64(1 << 60)
				if int64(int(big)) != big && (m.Name == "Int" || m.Name == "Intn") {
					// 32-bit machine cannot print 64-bit results
					val = "truncated"
				} else if reflect.TypeOf(out).Kind() == reflect.Slice {
					val = fmt.Sprintf("%#v", out)
				} else {
					val = fmt.Sprintf("%T(%v)", out, out)
				}
				fmt.Printf("\t%s, // %s(%s)\n", val, m.Name, argstr)
			} else {
				want := regressGolden[p]
				if m.Name == "Int" {
					want = int64(int(uint(want.(int64)) << 1 >> 1))
				}
				if !reflect.DeepEqual(out, want) {
					t.Errorf("r.%s(%s) = %v, want %v", m.Name, argstr, out, want)
				}
			}
			p++
		}
	}
	if *printgolden {
		fmt.Printf("}\n")
	}
}

var regressGolden = []interface{}{
	float64(0.6279600685109523),   // ExpFloat64()
	float64(0.16198826513357806),  // ExpFloat64()
	float64(0.007880404652650552), // ExpFloat64()
	float64(0.41649788761745654),  // ExpFloat64()
	float64(1.6958707787276301),   // ExpFloat64()
	float64(2.7227327706138036),   // ExpFloat64()
	float64(2.4235600263079657),   // ExpFloat64()
	float64(1.277967771105418),    // ExpFloat64()
	float64(0.7111660437031769),   // ExpFloat64()
	float64(0.23090401427981888),  // ExpFloat64()
	float64(1.4746763588379928),   // ExpFloat64()
	float64(1.4868726779832278),   // ExpFloat64()
	float64(0.1686257242078103),   // ExpFloat64()
	float64(0.2732721816228957),   // ExpFloat64()
	float64(0.4644536065869748),   // ExpFloat64()
	float64(0.01319850986379164),  // ExpFloat64()
	float64(0.7184492551742854),   // ExpFloat64()
	float64(0.1913536422195827),   // ExpFloat64()
	float64(0.16034475958495667),  // ExpFloat64()
	float64(0.40599859014785644),  // ExpFloat64()

	float32(0.7979972),   // Float32()
	float32(0.7725961),   // Float32()
	float32(0.21894403),  // Float32()
	float32(0.96194494),  // Float32()
	float32(0.2915732),   // Float32()
	float32(0.59569645),  // Float32()
	float32(0.99596655),  // Float32()
	float32(0.4979039),   // Float32()
	float32(0.98148686),  // Float32()
	float32(0.01380035),  // Float32()
	float32(0.086487144), // Float32()
	float32(0.6114401),   // Float32()
	float32(0.71081316),  // Float32()
	float32(0.6342346),   // Float32()
	float32(0.008082573), // Float32()
	float32(0.33020085),  // Float32()
	float32(0.032625034), // Float32()
	float32(0.9278005),   // Float32()
	float32(0.34497985),  // Float32()
	float32(0.66506875),  // Float32()

	float64(0.797997151016231),    // Float64()
	float64(0.7725961454373316),   // Float64()
	float64(0.21894402538580782),  // Float64()
	float64(0.9619449481780457),   // Float64()
	float64(0.2915731877602916),   // Float64()
	float64(0.5956964580775652),   // Float64()
	float64(0.9959665347028619),   // Float64()
	float64(0.49790390966591147),  // Float64()
	float64(0.9814868602566785),   // Float64()
	float64(0.013800350332924483), // Float64()
	float64(0.08648714463652596),  // Float64()
	float64(0.6114401479210267),   // Float64()
	float64(0.7108131531183706),   // Float64()
	float64(0.6342346133706837),   // Float64()
	float64(0.008082572853887138), // Float64()
	float64(0.3302008651926287),   // Float64()
	float64(0.03262503454637655),  // Float64()
	float64(0.9278004634858956),   // Float64()
	float64(0.3449798628384906),   // Float64()
	float64(0.665068719316529),    // Float64()

	int64(5474557666971700975), // Int()
	int64(5591422465364813936), // Int()
	int64(74029666500212977),   // Int()
	int64(8088122161323000979), // Int()
	int64(7298457654139700474), // Int()
	int64(1590632625527662686), // Int()
	int64(9052198920789078554), // Int()
	int64(7381380909356947872), // Int()
	int64(1738222704626512495), // Int()
	int64(3278744831230954970), // Int()
	int64(7062423222661652521), // Int()
	int64(6715870808026712034), // Int()
	int64(528819992478005418),  // Int()
	int64(2284534088986354339), // Int()
	int64(945828723091990082),  // Int()
	int64(3813019469742317492), // Int()
	int64(1369388146907482806), // Int()
	int64(7367238674766648970), // Int()
	int64(8217673022687244206), // Int()
	int64(3185531743396549562), // Int()

	int32(1711064216), // Int31()
	int32(650927245),  // Int31()
	int32(8618187),    // Int31()
	int32(941581344),  // Int31()
	int32(1923394120), // Int31()
	int32(1258915833), // Int31()
	int32(1053814650), // Int31()
	int32(859305834),  // Int31()
	int32(1276097579), // Int31()
	int32(1455437958), // Int31()
	int32(1895916096), // Int31()
	int32(781830261),  // Int31()
	int32(61562749),   // Int31()
	int32(265954771),  // Int31()
	int32(1183850779), // Int31()
	int32(443893888),  // Int31()
	int32(1233159585), // Int31()
	int32(857659461),  // Int31()
	int32(956663049),  // Int31()
	int32(370844703),  // Int31()

	int32(0),          // Int31n(1)
	int32(6),          // Int31n(10)
	int32(17),         // Int31n(32)
	int32(1000595),    // Int31n(1048576)
	int32(424333),     // Int31n(1048577)
	int32(382438494),  // Int31n(1000000000)
	int32(902738458),  // Int31n(1073741824)
	int32(1204933878), // Int31n(2147483646)
	int32(1376191263), // Int31n(2147483647)
	int32(0),          // Int31n(1)
	int32(9),          // Int31n(10)
	int32(2),          // Int31n(32)
	int32(440490),     // Int31n(1048576)
	int32(176312),     // Int31n(1048577)
	int32(946765890),  // Int31n(1000000000)
	int32(665034676),  // Int31n(1073741824)
	int32(1947285452), // Int31n(2147483646)
	int32(1702344608), // Int31n(2147483647)
	int32(0),          // Int31n(1)
	int32(2),          // Int31n(10)

	int64(5474557666971700975), // Int63()
	int64(5591422465364813936), // Int63()
	int64(74029666500212977),   // Int63()
	int64(8088122161323000979), // Int63()
	int64(7298457654139700474), // Int63()
	int64(1590632625527662686), // Int63()
	int64(9052198920789078554), // Int63()
	int64(7381380909356947872), // Int63()
	int64(1738222704626512495), // Int63()
	int64(3278744831230954970), // Int63()
	int64(7062423222661652521), // Int63()
	int64(6715870808026712034), // Int63()
	int64(528819992478005418),  // Int63()
	int64(2284534088986354339), // Int63()
	int64(945828723091990082),  // Int63()
	int64(3813019469742317492), // Int63()
	int64(1369388146907482806), // Int63()
	int64(7367238674766648970), // Int63()
	int64(8217673022687244206), // Int63()
	int64(3185531743396549562), // Int63()

	int64(0),                   // Int63n(1)
	int64(6),                   // Int63n(10)
	int64(17),                  // Int63n(32)
	int64(1000595),             // Int63n(1048576)
	int64(424333),              // Int63n(1048577)
	int64(382438494),           // Int63n(1000000000)
	int64(902738458),           // Int63n(1073741824)
	int64(1204933878),          // Int63n(2147483646)
	int64(1376191263),          // Int63n(2147483647)
	int64(502116868085730778),  // Int63n(1000000000000000000)
	int64(144894195020570665),  // Int63n(1152921504606846976)
	int64(6715870808026712034), // Int63n(9223372036854775806)
	int64(528819992478005418),  // Int63n(9223372036854775807)
	int64(0),                   // Int63n(1)
	int64(0),                   // Int63n(10)
	int64(20),                  // Int63n(32)
	int64(854710),              // Int63n(1048576)
	int64(649893),              // Int63n(1048577)
	int64(687244206),           // Int63n(1000000000)
	int64(836883386),           // Int63n(1073741824)

	int64(0),                   // Intn(1)
	int64(6),                   // Intn(10)
	int64(17),                  // Intn(32)
	int64(1000595),             // Intn(1048576)
	int64(424333),              // Intn(1048577)
	int64(382438494),           // Intn(1000000000)
	int64(902738458),           // Intn(1073741824)
	int64(1204933878),          // Intn(2147483646)
	int64(1376191263),          // Intn(2147483647)
	int64(502116868085730778),  // Intn(1000000000000000000)
	int64(144894195020570665),  // Intn(1152921504606846976)
	int64(6715870808026712034), // Intn(9223372036854775806)
	int64(528819992478005418),  // Intn(9223372036854775807)
	int64(0),                   // Intn(1)
	int64(0),                   // Intn(10)
	int64(20),                  // Intn(32)
	int64(854710),              // Intn(1048576)
	int64(649893),              // Intn(1048577)
	int64(687244206),           // Intn(1000000000)
	int64(836883386),           // Intn(1073741824)

	float64(-0.5410658516792047),  // NormFloat64()
	float64(0.615296849055287),    // NormFloat64()
	float64(0.007477442280032887), // NormFloat64()
	float64(1.3443892057169684),   // NormFloat64()
	float64(-0.17508902754863512), // NormFloat64()
	float64(-2.03494397556937),    // NormFloat64()
	float64(2.5213558871972306),   // NormFloat64()
	float64(1.4572921639613627),   // NormFloat64()
	float64(-1.5164961164210644),  // NormFloat64()
	float64(-0.4861150771891445),  // NormFloat64()
	float64(-0.8699409548614199),  // NormFloat64()
	float64(1.6271559815452794),   // NormFloat64()
	float64(0.1659465769926195),   // NormFloat64()
	float64(0.2921716191987018),   // NormFloat64()
	float64(-1.2550269636927838),  // NormFloat64()
	float64(0.11257973349467548),  // NormFloat64()
	float64(0.5437525915836436),   // NormFloat64()
	float64(0.781754430770282),    // NormFloat64()
	float64(0.5201256313962235),   // NormFloat64()
	float64(1.3826174159276245),   // NormFloat64()

	[]int{},                             // Perm(0)
	[]int{0},                            // Perm(1)
	[]int{0, 2, 3, 1, 4},                // Perm(5)
	[]int{5, 6, 3, 7, 4, 2, 0, 1},       // Perm(8)
	[]int{8, 4, 5, 2, 7, 3, 0, 6, 1},    // Perm(9)
	[]int{6, 1, 5, 3, 2, 9, 7, 0, 8, 4}, // Perm(10)
	[]int{12, 5, 1, 9, 15, 7, 13, 6, 10, 11, 8, 0, 4, 2, 14, 3}, // Perm(16)
	[]int{},                             // Perm(0)
	[]int{0},                            // Perm(1)
	[]int{0, 2, 3, 4, 1},                // Perm(5)
	[]int{3, 2, 7, 4, 0, 6, 5, 1},       // Perm(8)
	[]int{0, 6, 2, 1, 3, 7, 5, 8, 4},    // Perm(9)
	[]int{2, 5, 6, 4, 7, 3, 0, 8, 1, 9}, // Perm(10)
	[]int{3, 6, 5, 4, 9, 15, 13, 7, 1, 11, 10, 8, 12, 0, 2, 14}, // Perm(16)
	[]int{},                             // Perm(0)
	[]int{0},                            // Perm(1)
	[]int{2, 4, 3, 1, 0},                // Perm(5)
	[]int{1, 6, 7, 5, 4, 3, 2, 0},       // Perm(8)
	[]int{7, 6, 8, 2, 0, 1, 3, 4, 5},    // Perm(9)
	[]int{2, 9, 7, 1, 5, 4, 0, 6, 8, 3}, // Perm(10)

	[]byte{0xef}, // Read([0])
	[]byte{0x4e, 0x3d, 0x52, 0x31, 0x89, 0xf9, 0xcb},                   // Read([0 0 0 0 0 0 0])
	[]byte{0x70, 0x68, 0x35, 0x8d, 0x1b, 0xb9, 0x98, 0x4d},             // Read([0 0 0 0 0 0 0 0])
	[]byte{0xf1, 0xf8, 0x95, 0xe6, 0x96, 0x1, 0x7, 0x1, 0x93},          // Read([0 0 0 0 0 0 0 0 0])
	[]byte{0x44, 0x9f, 0xc5, 0x40, 0xc8, 0x3e, 0x70, 0xfa, 0x44, 0x3a}, // Read([0 0 0 0 0 0 0 0 0 0])
	[]byte{0x4b}, // Read([0])
	[]byte{0x91, 0x54, 0x49, 0xe5, 0x5e, 0x28, 0xb9},                   // Read([0 0 0 0 0 0 0])
	[]byte{0x4, 0xf2, 0xf, 0x13, 0x96, 0x1a, 0xb2, 0xce},               // Read([0 0 0 0 0 0 0 0])
	[]byte{0x35, 0xf5, 0xde, 0x9f, 0x7d, 0xa0, 0x19, 0x12, 0x2e},       // Read([0 0 0 0 0 0 0 0 0])
	[]byte{0xd4, 0xee, 0x6f, 0x66, 0x6f, 0x32, 0xc8, 0x21, 0x57, 0x68}, // Read([0 0 0 0 0 0 0 0 0 0])
	[]byte{0x1f}, // Read([0])
	[]byte{0x98, 0xda, 0x4d, 0xab, 0x6e, 0xd, 0x71},                   // Read([0 0 0 0 0 0 0])
	[]byte{0x80, 0xad, 0x29, 0xa0, 0x37, 0xb0, 0x80, 0xc4},            // Read([0 0 0 0 0 0 0 0])
	[]byte{0x2, 0xe2, 0xe2, 0x7, 0xd9, 0xed, 0xea, 0x90, 0x33},        // Read([0 0 0 0 0 0 0 0 0])
	[]byte{0x5d, 0xaa, 0xb8, 0xc6, 0x39, 0xfb, 0xbe, 0x56, 0x7, 0xa3}, // Read([0 0 0 0 0 0 0 0 0 0])
	[]byte{0x62}, // Read([0])
	[]byte{0x4d, 0x63, 0xa6, 0x4b, 0xb4, 0x1f, 0x42},                // Read([0 0 0 0 0 0 0])
	[]byte{0x66, 0x42, 0x62, 0x36, 0x42, 0x20, 0x8d, 0xb4},          // Read([0 0 0 0 0 0 0 0])
	[]byte{0x9f, 0xa3, 0x67, 0x1, 0x91, 0xea, 0x34, 0xb6, 0xa},      // Read([0 0 0 0 0 0 0 0 0])
	[]byte{0xd, 0xa8, 0x43, 0xb, 0x1, 0x93, 0x8a, 0x56, 0xfc, 0x98}, // Read([0 0 0 0 0 0 0 0 0 0])

	uint32(3422128433), // Uint32()
	uint32(1301854491), // Uint32()
	uint32(17236374),   // Uint32()
	uint32(1883162688), // Uint32()
	uint32(3846788241), // Uint32()
	uint32(2517831666), // Uint32()
	uint32(2107629301), // Uint32()
	uint32(1718611668), // Uint32()
	uint32(2552195159), // Uint32()
	uint32(2910875917), // Uint32()
	uint32(3791832192), // Uint32()
	uint32(1563660522), // Uint32()
	uint32(123125499),  // Uint32()
	uint32(531909542),  // Uint32()
	uint32(2367701558), // Uint32()
	uint32(887787777),  // Uint32()
	uint32(2466319171), // Uint32()
	uint32(1715318922), // Uint32()
	uint32(1913326099), // Uint32()
	uint32(741689406),  // Uint32()

	uint64(14697929703826476783), // Uint64()
	uint64(5591422465364813936),  // Uint64()
	uint64(74029666500212977),    // Uint64()
	uint64(8088122161323000979),  // Uint64()
	uint64(16521829690994476282), // Uint64()
	uint64(10814004662382438494), // Uint64()
	uint64(9052198920789078554),  // Uint64()
	uint64(7381380909356947872),  // Uint64()
	uint64(10961594741481288303), // Uint64()
	uint64(12502116868085730778), // Uint64()
	uint64(16285795259516428329), // Uint64()
	uint64(6715870808026712034),  // Uint64()
	uint64(528819992478005418),   // Uint64()
	uint64(2284534088986354339),  // Uint64()
	uint64(10169200759946765890), // Uint64()
	uint64(3813019469742317492),  // Uint64()
	uint64(10592760183762258614), // Uint64()
	uint64(7367238674766648970),  // Uint64()
	uint64(8217673022687244206),  // Uint64()
	uint64(3185531743396549562),  // Uint64()

	uint64(0),                   // Uint64n(1)
	uint64(6),                   // Uint64n(10)
	uint64(17),                  // Uint64n(32)
	uint64(1000595),             // Uint64n(1048576)
	uint64(424333),              // Uint64n(1048577)
	uint64(382438494),           // Uint64n(1000000000)
	uint64(902738458),           // Uint64n(1073741824)
	uint64(1204933878),          // Uint64n(2147483646)
	uint64(1376191263),          // Uint64n(2147483647)
	uint64(502116868085730778),  // Uint64n(1000000000000000000)
	uint64(144894195020570665),  // Uint64n(1152921504606846976)
	uint64(6715870808026712034), // Uint64n(18446744073709551614)
	uint64(528819992478005418),  // Uint64n(18446744073709551615)
	uint64(0),                   // Uint64n(1)
	uint64(0),                   // Uint64n(10)
	uint64(20),                  // Uint64n(32)
	uint64(854710),              // Uint64n(1048576)
	uint64(649893),              // Uint64n(1048577)
	uint64(687244206),           // Uint64n(1000000000)
	uint64(836883386),           // Uint64n(1073741824)
}
