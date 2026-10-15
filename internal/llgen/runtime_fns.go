package llgen

import (
	"fmt"
	"strings"
)

// rtInit and the rest are the only place a call's arity and types are
// decided; runtime_fns_test.go checks them against the C definitions.
var (
	rtInit         = RuntimeFn{"rt_init", []Type{TI32, TPtr, TI32, TPtr}, TVoid}
	rtAbiCheck     = RuntimeFn{"rt_abi_check", []Type{TPtr}, TVoid}
	rtExitCode     = RuntimeFn{"rt_exit_code", []Type{TPtr}, TI32}
	rtUnit         = RuntimeFn{"rt_unit", nil, TPtr}
	rtInt          = RuntimeFn{"rt_int", []Type{TI64, TI32}, TPtr}
	rtFloat        = RuntimeFn{"rt_float", []Type{TDouble, TI32}, TPtr}
	rtBool         = RuntimeFn{"rt_bool", []Type{TI32}, TPtr}
	rtChar         = RuntimeFn{"rt_char", []Type{TI32}, TPtr}
	rtStr          = RuntimeFn{"rt_str", []Type{TPtr, TI64}, TPtr}
	rtBytes        = RuntimeFn{"rt_bytes", []Type{TPtr, TI64}, TPtr}
	rtCopy         = RuntimeFn{"rt_copy", []Type{TPtr}, TPtr}
	rtCopyMut      = RuntimeFn{"rt_copy_mut", []Type{TPtr}, TPtr}
	rtTruth        = RuntimeFn{"rt_truth", []Type{TPtr}, TI1}
	rtAt           = RuntimeFn{"rt_at", []Type{TPtr, TI64}, TPtr}
	rtCallv        = RuntimeFn{"rt_callv", []Type{TPtr, TI32, TPtr}, TPtr}
	rtBuiltin      = RuntimeFn{"rt_builtin", []Type{TPtr, TI32, TPtr}, TPtr}
	rtStd          = RuntimeFn{"rt_std", []Type{TPtr, TI32, TPtr}, TPtr}
	rtStdId        = RuntimeFn{"rt_std_id", []Type{TI32, TI32, TPtr}, TPtr}
	rtStdClosure   = RuntimeFn{"rt_std_closure", []Type{TPtr, TI32, TPtr}, TPtr}
	rtStdBind      = RuntimeFn{"rt_std_bind", []Type{TPtr, TI32, TI32, TPtr}, TPtr}
	rtCallSlot     = RuntimeFn{"rt_call_slot", []Type{TPtr, TI32, TI32, TPtr, TI32}, TPtr}
	rtBinop        = RuntimeFn{"rt_binop", []Type{TPtr, TPtr, TPtr}, TPtr}
	rtUnop         = RuntimeFn{"rt_unop", []Type{TPtr, TPtr}, TPtr}
	rtCast         = RuntimeFn{"rt_cast", []Type{TPtr, TI32}, TPtr}
	rtIadd         = RuntimeFn{"rt_iadd", []Type{TI64, TI64, TI32}, TI64}
	rtIsub         = RuntimeFn{"rt_isub", []Type{TI64, TI64, TI32}, TI64}
	rtImul         = RuntimeFn{"rt_imul", []Type{TI64, TI64, TI32}, TI64}
	rtIdiv         = RuntimeFn{"rt_idiv", []Type{TI64, TI64, TI32}, TI64}
	rtImod         = RuntimeFn{"rt_imod", []Type{TI64, TI64, TI32}, TI64}
	rtIxor         = RuntimeFn{"rt_ixor", []Type{TI64, TI64, TI32}, TI64}
	rtIshl         = RuntimeFn{"rt_ishl", []Type{TI64, TI64, TI32}, TI64}
	rtIshr         = RuntimeFn{"rt_ishr", []Type{TI64, TI64, TI32}, TI64}
	rtIneg         = RuntimeFn{"rt_ineg", []Type{TI64, TI32}, TI64}
	rtInot         = RuntimeFn{"rt_inot", []Type{TI64, TI32}, TI64}
	rtRecord       = RuntimeFn{"rt_record", []Type{TPtr, TI64, TPtr}, TPtr}
	rtVariant      = RuntimeFn{"rt_variant", []Type{TPtr, TI64, TPtr}, TPtr}
	rtNullary      = RuntimeFn{"rt_nullary", []Type{TPtr}, TPtr}
	rtConstVariant = RuntimeFn{"rt_const_variant", []Type{TPtr, TPtr, TPtr, TI64}, TVoid}
	rtConstNum     = RuntimeFn{"rt_const_num", []Type{TPtr, TI32, TI32, TI64, TDouble}, TVoid}
	rtConstInt128  = RuntimeFn{"rt_const_int128", []Type{TPtr, TI32, TI64, TI64}, TVoid}
	rtBoxInt128    = RuntimeFn{"rt_box_int128", []Type{TI64, TI64, TI32}, TPtr}
	rtInt128Lo     = RuntimeFn{"rt_int128_lo", []Type{TPtr}, TI64}
	rtInt128Hi     = RuntimeFn{"rt_int128_hi", []Type{TPtr}, TI64}
	rtI128Mul      = RuntimeFn{"rt_i128_mul", []Type{TI64, TI64, TI64, TI64, TI32, TPtr}, TVoid}
	rtI128Div      = RuntimeFn{"rt_i128_div", []Type{TI64, TI64, TI64, TI64, TI32, TPtr}, TVoid}
	rtConstText    = RuntimeFn{"rt_const_text", []Type{TPtr, TI32, TPtr, TI64}, TVoid}
	rtInitNullary  = RuntimeFn{"rt_init_nullary", []Type{TI32, TPtr}, TVoid}
	rtField        = RuntimeFn{"rt_field", []Type{TPtr, TI64}, TPtr}
	rtFieldMove    = RuntimeFn{"rt_field_move", []Type{TPtr, TI64}, TPtr}
	rtSetfield     = RuntimeFn{"rt_setfield", []Type{TPtr, TI64, TPtr}, TVoid}
	rtArray        = RuntimeFn{"rt_array", []Type{TI64, TPtr}, TPtr}
	rtIndex        = RuntimeFn{"rt_index", []Type{TPtr, TPtr}, TPtr}
	rtSetindex     = RuntimeFn{"rt_setindex", []Type{TPtr, TPtr, TPtr}, TVoid}
	rtSetindexCopy = RuntimeFn{"rt_setindex_copy", []Type{TPtr, TPtr, TPtr}, TVoid}
	rtPayload      = RuntimeFn{"rt_payload", []Type{TPtr, TI64}, TPtr}
	rtIsvariant    = RuntimeFn{"rt_isvariant", []Type{TPtr, TPtr}, TPtr}
	rtVariantIndex = RuntimeFn{"rt_variant_index", []Type{TPtr}, TI32}
	rtIstype       = RuntimeFn{"rt_istype", []Type{TPtr, TPtr}, TPtr}
	rtUnbox        = RuntimeFn{"rt_unbox", []Type{TPtr}, TPtr}
	rtBox          = RuntimeFn{"rt_box", []Type{TPtr, TPtr, TPtr}, TPtr}
	rtClosure      = RuntimeFn{"rt_closure", []Type{TPtr, TI32, TPtr}, TPtr}
	rtClosureAsync = RuntimeFn{"rt_closure_async", []Type{TPtr, TI32, TPtr}, TPtr}
	rtClosureEnv   = RuntimeFn{"rt_closure_env", []Type{TPtr}, TPtr}
	rtArrayPush    = RuntimeFn{"rt_array_push", []Type{TPtr, TPtr}, TVoid}
	rtFnItemInit   = RuntimeFn{"rt_fnitem_init", []Type{TPtr, TPtr, TI32}, TVoid}
	rtStdItemInit  = RuntimeFn{"rt_stditem_init", []Type{TPtr, TPtr, TI32}, TVoid}
	rtCell         = RuntimeFn{"rt_cell", []Type{TPtr}, TPtr}
	rtCellInit     = RuntimeFn{"rt_cell_init", []Type{TPtr, TPtr}, TVoid}
	rtCellget      = RuntimeFn{"rt_cellget", []Type{TPtr}, TPtr}
	rtCellTake     = RuntimeFn{"rt_cell_take", []Type{TPtr}, TPtr}
	rtCellset      = RuntimeFn{"rt_cellset", []Type{TPtr, TPtr}, TVoid}
	rtInterp       = RuntimeFn{"rt_interp", []Type{TI32, TPtr}, TPtr}
	rtDisplay      = RuntimeFn{"display", []Type{TPtr}, TPtr}
	rtDrop         = RuntimeFn{"rt_drop", []Type{TPtr}, TVoid}
	rtDropOuter    = RuntimeFn{"rt_drop_outer", []Type{TPtr}, TVoid}
	rtReleaseOuter = RuntimeFn{"rt_release_outer", []Type{TPtr}, TI32}
	rtSweepRelease = RuntimeFn{"rt_sweep_release", []Type{TPtr}, TVoid}
	rtOuterFatal   = RuntimeFn{"rt_outer_fatal", []Type{}, TVoid}
	rtRetain       = RuntimeFn{"rt_retain", []Type{TPtr}, TPtr}
	rtRelease      = RuntimeFn{"rt_release", []Type{TPtr}, TVoid}
	rtPanic        = RuntimeFn{"rt_panic", []Type{TPtr}, TVoid}
	rtIntVal       = RuntimeFn{"rt_int_val", []Type{TPtr}, TI64}
	rtPtr          = RuntimeFn{"rt_ptr", []Type{TPtr}, TPtr}
	rtConvert      = RuntimeFn{"rt_convert", []Type{TPtr, TI32, TI32}, TPtr}
	rtPtrVal       = RuntimeFn{"rt_ptr_val", []Type{TPtr}, TPtr}
	rtFloatVal     = RuntimeFn{"rt_float_val", []Type{TPtr}, TDouble}
	rtCharVal      = RuntimeFn{"rt_char_val", []Type{TPtr}, TI32}
	rtCstr         = RuntimeFn{"rt_cstr", []Type{TPtr}, TPtr}
	rtFromCstr     = RuntimeFn{"rt_from_cstr", []Type{TPtr}, TPtr}
	rtErrorAs      = RuntimeFn{"rt_error_as", []Type{TPtr, TPtr, TPtr, TI64}, TPtr}

	rtFutureNew        = RuntimeFn{"rt_future_new", []Type{TPtr, TI32, TPtr}, TPtr}
	rtFutureSlots      = RuntimeFn{"rt_future_slots", []Type{TPtr}, TPtr}
	rtFutureState      = RuntimeFn{"rt_future_state", []Type{TPtr}, TI32}
	rtFutureArg        = RuntimeFn{"rt_future_arg", []Type{TPtr, TI64}, TPtr}
	rtFutureStarted    = RuntimeFn{"rt_future_started", []Type{TPtr}, TVoid}
	rtFutureSuspend    = RuntimeFn{"rt_future_suspend", []Type{TPtr, TI32, TPtr}, TVoid}
	rtFutureResume     = RuntimeFn{"rt_future_resume", []Type{TPtr}, TPtr}
	rtFutureComplete   = RuntimeFn{"rt_future_complete", []Type{TPtr, TPtr}, TPtr}
	rtUnwindEnter      = RuntimeFn{"rt_unwind_enter", []Type{TPtr, TI64}, TVoid}
	rtUnwindEnterSlots = RuntimeFn{"rt_unwind_enter_slots", []Type{TPtr, TPtr, TPtr, TI64}, TVoid}
	rtUnwindLeave      = RuntimeFn{"rt_unwind_leave", []Type{TPtr}, TVoid}

	rtSetfieldSwap     = RuntimeFn{"rt_setfield_swap", []Type{TPtr, TI64, TPtr}, TPtr}
	rtSetindexSwap     = RuntimeFn{"rt_setindex_swap", []Type{TPtr, TPtr, TPtr}, TPtr}
	rtSetindexCopySwap = RuntimeFn{"rt_setindex_copy_swap", []Type{TPtr, TPtr, TPtr}, TPtr}
	rtCellsetSwap      = RuntimeFn{"rt_cellset_swap", []Type{TPtr, TPtr}, TPtr}
)

// runtimeFns lists every RuntimeFn above in `declare` order: the single
// source renderExterns and runtime_fns_test.go both read.
var runtimeFns = []RuntimeFn{
	rtInit, rtAbiCheck, rtExitCode, rtUnit, rtInt, rtFloat, rtBool, rtChar, rtStr, rtBytes, rtCopy, rtCopyMut,
	rtTruth, rtAt, rtCallv, rtBuiltin, rtStd, rtStdId, rtStdClosure, rtStdBind, rtCallSlot,
	rtBinop, rtUnop, rtCast, rtIadd, rtIsub, rtImul, rtIdiv, rtImod, rtIxor, rtIshl, rtIshr, rtIneg, rtInot,
	rtRecord, rtVariant, rtNullary, rtConstVariant, rtConstNum, rtConstInt128, rtBoxInt128, rtInt128Lo, rtInt128Hi, rtI128Mul, rtI128Div, rtConstText, rtInitNullary, rtField, rtFieldMove, rtSetfield, rtArray,
	rtIndex, rtSetindex, rtSetindexCopy, rtPayload, rtIsvariant, rtVariantIndex, rtIstype, rtUnbox, rtBox,
	rtClosure, rtClosureAsync, rtClosureEnv, rtArrayPush, rtFnItemInit, rtStdItemInit, rtCell, rtCellInit, rtCellget, rtCellTake, rtCellset, rtInterp, rtDisplay, rtDrop,
	rtRetain, rtRelease, rtPanic, rtIntVal, rtPtr, rtConvert, rtPtrVal,
	rtFloatVal, rtCstr, rtFromCstr, rtCharVal, rtErrorAs,
	rtFutureNew, rtFutureSlots, rtFutureState, rtFutureArg, rtFutureStarted,
	rtFutureSuspend, rtFutureResume, rtFutureComplete, rtUnwindEnter, rtUnwindEnterSlots, rtUnwindLeave,
	rtDropOuter, rtReleaseOuter, rtSweepRelease, rtOuterFatal,
	rtSetfieldSwap, rtSetindexSwap, rtSetindexCopySwap, rtCellsetSwap,
	rtDLen, rtDArrayLen, rtDBytesGet, rtDArrayGet, rtDCharFromInt, rtDParseFloat, rtDConvertInt, rtDConvertFloat,
	rtDWrapping, rtDSqrt, rtDFloor, rtDCeil, rtDRound, rtDPow,
	rtDHashInt, rtDHashBool, rtDHashChar, rtDHashFloat,
}

// renderExterns renders the `declare` list, one line per RuntimeFn in
// table order.
func renderExterns() string {
	var sb strings.Builder
	for _, fn := range runtimeFns {
		params := make([]string, len(fn.Params))
		for i, p := range fn.Params {
			params[i] = p.String()
		}
		fmt.Fprintf(&sb, "declare %s @%s(%s)\n", fn.Ret, fn.Name, strings.Join(params, ", "))
	}
	sb.WriteString("\n")
	return sb.String()
}
