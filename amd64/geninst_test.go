package amd64

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"

	"github.com/nelhage/gojit"
	"golang.org/x/arch/x86/x86asm"
)

var mem []byte = make([]byte, 64)

type simple struct {
	f     func(*Assembler)
	inout []uintptr
}

func TestMov(t *testing.T) {
	cases := []simple{
		{
			func(a *Assembler) {
				a.Mov(Imm{U32(0xdeadbeef)}, Rax)
			},
			[]uintptr{0, 0xdeadbeef},
		},
		{
			func(a *Assembler) {
				a.Mov(Rdi, Rax)
			},
			[]uintptr{0, 0, 1, 1, 0xdeadbeef, 0xdeadbeef, 0xffffffffffffffff, 0xffffffffffffffff},
		},
		{
			func(a *Assembler) {
				a.Mov(Imm{U32(0xcafebabe)}, Indirect{Rdi, 0, 64})
				a.Mov(Indirect{Rdi, 0, 64}, Rax)
			},
			[]uintptr{gojit.Addr(mem), 0xffffffffcafebabe},
		},
		{
			func(a *Assembler) {
				a.Mov(Imm{U32(0xf00dface)}, R10)
				a.Mov(R10, Rax)
			},
			[]uintptr{0, 0xf00dface},
		},
	}

	testSimple("mov", t, cases)
}

func TestIncDec(t *testing.T) {
	cases := []simple{
		{
			func(a *Assembler) {
				a.Mov(Rdi, Rax)
				a.Inc(Rax)
			},
			[]uintptr{0, 1, 10, 11},
		},
		{
			func(a *Assembler) {
				a.Mov(Rdi, Rax)
				a.Dec(Rax)
			},
			[]uintptr{1, 0, 11, 10},
		},
		{
			func(a *Assembler) {
				a.Mov(Imm{0x11223344}, Indirect{Rdi, 0, 32})
				a.Incb(Indirect{Rdi, 1, 8})
				a.Mov(Indirect{Rdi, 0, 32}, Eax)
			},
			[]uintptr{gojit.Addr(mem), 0x11223444},
		},
		{
			func(a *Assembler) {
				a.Mov(Imm{0x11223344}, Indirect{Rdi, 0, 32})
				a.Decb(Indirect{Rdi, 1, 8})
				a.Mov(Indirect{Rdi, 0, 32}, Eax)
			},
			[]uintptr{gojit.Addr(mem), 0x11223244},
		},
	}
	testSimple("inc/dec", t, cases)
}

func testSimple(name string, t *testing.T, cases []simple) {
	buf, e := gojit.Alloc(gojit.PageSize)
	if e != nil {
		t.Fatalf(e.Error())
	}
	defer gojit.Release(buf)

	for i, tc := range cases {
		asm := &Assembler{Buf: buf}
		begin(asm)
		tc.f(asm)
		f := finish(asm)

		runtime.GC()

		for j := 0; j < len(tc.inout); j += 2 {
			in := tc.inout[j]
			out := tc.inout[j+1]
			got := f(in)
			if out != got {
				t.Errorf("f(%s)[%d](%x) = %x, expect %x",
					name, i, in, got, out)
			}
		}
	}
}

func TestEmitArith(t *testing.T) {
	var asm *Assembler

	tests := []struct {
		Emit func()
		Exp  string
	}{
		{func() { asm.Add(Imm{-5}, R14) }, "0000  ADDQ $-0x5, R14\n"},
		{func() { asm.Add(Imm{-5}, Rdi) }, "0000  ADDQ $-0x5, DI\n"},
		{func() { asm.Add(Imm{-5}, Edi) }, "0000  ADDL $-0x5, DI\n"},
		{func() { asm.Shl(Imm{10}, R8) }, "0000  SHLQ $0xa, R8\n"},
		{func() { asm.Shl(Imm{10}, Rbx) }, "0000  SHLQ $0xa, BX\n"},
		{func() { asm.Shl(Imm{10}, Ebx) }, "0000  SHLL $0xa, BX\n"},
		{func() { asm.ShlCl(R10) }, "0000  SHLQ CL, R10\n"},
		{func() { asm.ShlCl(R10d) }, "0000  SHLL CL, R10\n"},
		{func() { asm.ShrCl(R10) }, "0000  SHRQ CL, R10\n"},
		{func() { asm.ShrCl(R10d) }, "0000  SHRL CL, R10\n"},
		{func() { asm.SarCl(R10) }, "0000  SARQ CL, R10\n"},
		{func() { asm.SarCl(R10d) }, "0000  SARL CL, R10\n"},
		{func() { asm.SarCl(Eax) }, "0000  SARL CL, AX\n"},
		{func() { asm.Rol(Imm{10}, R8) }, "0000  ROLQ $0xa, R8\n"},
		{func() { asm.Ror(Imm{10}, R8) }, "0000  RORQ $0xa, R8\n"},
		{func() { asm.Rcl(Imm{10}, R8) }, "0000  RCLQ $0xa, R8\n"},
		{func() { asm.Rcr(Imm{10}, R8) }, "0000  RCRQ $0xa, R8\n"},
		{func() { asm.RolCl(R8) }, "0000  ROLQ CL, R8\n"},
		{func() { asm.RorCl(R8) }, "0000  RORQ CL, R8\n"},
		{func() { asm.RclCl(R8) }, "0000  RCLQ CL, R8\n"},
		{func() { asm.RcrCl(R8) }, "0000  RCRQ CL, R8\n"},
		{func() { asm.Test(Imm{5}, R14) }, "0000  TESTQ $0x5, R14\n"},
		{func() { asm.Test(Imm{5}, Rdi) }, "0000  TESTQ $0x5, DI\n"},
		{func() { asm.Test(Imm{5}, R14d) }, "0000  TESTL $0x5, R14\n"},
		{func() { asm.Test(Imm{5}, Edi) }, "0000  TESTL $0x5, DI\n"},
		{func() { asm.Testb(Imm{5}, Edx) }, "0000  TESTL $0x5, DL\n"},
		{func() { asm.Test(Rax, R14) }, "0000  TESTQ AX, R14\n"},
		{func() { asm.Test(Rax, Rdi) }, "0000  TESTQ AX, DI\n"},
		{func() { asm.Test(Eax, R14d) }, "0000  TESTL AX, R14\n"},
		{func() { asm.Test(Eax, Edi) }, "0000  TESTL AX, DI\n"},
		{func() { asm.Testb(Eax, Edx) }, "0000  TESTL AL, DL\n"},
		{func() { asm.Cmovcc(CC_GE, Eax, Edx) }, "0000  CMOVGE AX, DX\n"},
		{func() { asm.Cmovcc(CC_GE, Rax, Rdx) }, "0000  CMOVGE AX, DX\n"}, // FIXME: can't test 64-bit/32-bit version
		{func() { asm.Cmovcc(CC_A, R12, R14) }, "0000  CMOVA R12, R14\n"},
		{func() { asm.Cmovcc(CC_A, R12d, R14d) }, "0000  CMOVA R12, R14\n"}, // FIXME: can't test 64-bit/32-bit version
	}

	for _, tc := range tests {
		asm = &Assembler{mem, 0, CgoABI}
		tc.Emit()

		var got bytes.Buffer
		x86bin := mem[:asm.Off]
		pc := uint64(0)
		for len(x86bin) > 0 {
			inst, err := x86asm.Decode(x86bin, 64)
			var text string
			size := inst.Len
			if err != nil || size == 0 || inst.Op == 0 {
				size = 1
				text = "?"
			} else {
				text = x86asm.GoSyntax(inst, pc, nil)
			}

			fmt.Fprintf(&got, "%04x  %s\n", pc, text)
			x86bin = x86bin[size:]
			pc += uint64(size)
		}

		if got.String() != tc.Exp {
			t.Errorf("code generation failed: %x", mem[:asm.Off])
			t.Log("Got:")
			t.Log(got.String())
			t.Log("Expected")
			t.Log(tc.Exp)
		}
	}
}

func TestArith(t *testing.T) {
	cases := []struct {
		insn     *Instruction
		lhs, rhs int32
		out      uintptr
	}{
		{InstAdd, 20, 30, 50},
		{InstAdd, 0x7fffffff, 0x70000001, 0xf0000000},
		{InstAnd, 0x77777777, U32(0xffffffff), 0x77777777},
		{InstAnd, 0x77777777, U32(0x88888888), 0},
		{InstOr, 0x77777777, U32(0x88888888), 0xffffffff},
		{InstOr, 1, 0, 1},
		{InstSub, 5, 10, 5},
		{InstSub, 10, 5, 0xfffffffffffffffb},
	}

	buf, e := gojit.Alloc(gojit.PageSize)
	if e != nil {
		t.Fatalf(e.Error())
	}
	defer gojit.Release(buf)

	for _, tc := range cases {
		asm := &Assembler{buf, 0, CgoABI}
		var funcs []func(uintptr) uintptr
		if tc.insn.imm_r != nil {
			begin(asm)
			asm.Mov(Imm{tc.rhs}, Rax)
			asm.Arithmetic(tc.insn, Imm{tc.lhs}, Rax)
			funcs = append(funcs, finish(asm))
		}
		if tc.insn.imm_rm.op != nil {
			begin(asm)
			asm.Mov(Imm{0}, Indirect{Rdi, 0, 0})
			asm.Mov(Imm{tc.rhs}, Indirect{Rdi, 0, 32})
			asm.Arithmetic(tc.insn, Imm{tc.lhs}, Indirect{Rdi, 0, 64})
			asm.Mov(Indirect{Rdi, 0, 64}, Rax)
			funcs = append(funcs, finish(asm))
		}
		if tc.insn.r_rm != nil {
			begin(asm)
			asm.Mov(Imm{tc.lhs}, R10)
			asm.Mov(Imm{0}, Indirect{Rdi, 0, 0})
			asm.Mov(Imm{tc.rhs}, Indirect{Rdi, 0, 32})
			asm.Arithmetic(tc.insn, R10, Indirect{Rdi, 0, 64})
			asm.Mov(Indirect{Rdi, 0, 64}, Rax)
			funcs = append(funcs, finish(asm))
		}
		if tc.insn.rm_r != nil {
			begin(asm)
			asm.Mov(Imm{0}, Indirect{Rdi, 0, 0})
			asm.Mov(Imm{tc.lhs}, Indirect{Rdi, 0, 32})
			asm.Mov(Imm{tc.rhs}, R10)
			asm.Arithmetic(tc.insn, Indirect{Rdi, 0, 64}, R10)
			asm.Mov(R10, Rax)
			funcs = append(funcs, finish(asm))
		}

		for i, f := range funcs {
			got := f(gojit.Addr(mem))
			if got != tc.out {
				t.Errorf("%s(0x%x,0x%x) [%d] = 0x%x (expect 0x%x)",
					tc.insn.Mnemonic, tc.lhs, tc.rhs, i, got, tc.out)
			} else if testing.Verbose() {
				// We don't use `testing.Logf` because
				// if we panic inside JIT'd code, the
				// runtime dies horrible (rightfully
				// so!), and so the `testing` cleanup
				// code never runs, and we never see
				// log messages. We want to get these
				// out as soon as possible, so we
				// write them directly.
				fmt.Printf("OK %d %s(0x%x,0x%x) = 0x%x\n",
					i, tc.insn.Mnemonic, tc.lhs, tc.rhs, got)
			}
		}
	}
}

func TestMovEsp(t *testing.T) {
	asm := newAsm(t)
	defer gojit.Release(asm.Buf)

	begin(asm)

	// 48 c7 44 24 f8 69 7a 	movq   $0x7a69,-0x8(%rsp)
	// 00 00
	mov_rsp := []byte{0x48, 0xc7, 0x44, 0x24, 0xf8, 0x69, 0x7a, 0x00, 0x00}

	copy(asm.Buf[asm.Off:], mov_rsp)
	asm.Off += len(mov_rsp)
	asm.Mov(Indirect{Rsp, -8, 64}, Rax)
	f := finish(asm)

	got := f(0)
	if got != 31337 {
		t.Errorf("Fatal: mov from esp: got %d != %d", got, 31337)
	}
}
