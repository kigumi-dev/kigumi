package llgen

// rtInit and the rest are the only place a call's arity and types are
// decided; runtime_fns_test.go checks them against the C definitions.
var (
	rtInit                = RuntimeFn{Name: "rt_init", Params: []Type{TI32, TPtr, TI32, TPtr}, Ret: TVoid}
	rtAbiCheck            = RuntimeFn{Name: "rt_abi_check", Params: []Type{TPtr}, Ret: TVoid}
	rtExitCode            = RuntimeFn{Name: "rt_exit_code", Params: []Type{TPtr, TPtr, TPtr, TI64, TI64}, Ret: TI32}
	rtUnit                = RuntimeFn{Name: "rt_unit", Params: nil, Ret: TPtr}
	rtInt                 = RuntimeFn{Name: "rt_int", Params: []Type{TI64, TI32}, Ret: TPtr}
	rtFloat               = RuntimeFn{Name: "rt_float", Params: []Type{TDouble, TI32}, Ret: TPtr}
	rtBool                = RuntimeFn{Name: "rt_bool", Params: []Type{TI32}, Ret: TPtr}
	rtChar                = RuntimeFn{Name: "rt_char", Params: []Type{TI32}, Ret: TPtr}
	rtStr                 = RuntimeFn{Name: "rt_str", Params: []Type{TPtr, TI64}, Ret: TPtr}
	rtBytes               = RuntimeFn{Name: "rt_bytes", Params: []Type{TPtr, TI64}, Ret: TPtr}
	rtCopy                = RuntimeFn{Name: "rt_copy", Params: []Type{TPtr}, Ret: TPtr}
	rtCopyMut             = RuntimeFn{Name: "rt_copy_mut", Params: []Type{TPtr}, Ret: TPtr}
	rtTruth               = RuntimeFn{Name: "rt_truth", Params: []Type{TPtr}, Ret: TI1}
	rtAt                  = RuntimeFn{Name: "rt_at", Params: []Type{TPtr, TI64}, Ret: TPtr}
	rtCallv               = RuntimeFn{Name: "rt_callv", Params: []Type{TPtr, TI32, TPtr}, Ret: TPtr}
	rtCallvI64            = RuntimeFn{Name: "rt_callv_i64", Params: []Type{TPtr, TI32, TPtr}, Ret: TI64}
	rtIterCount           = RuntimeFn{Name: "rt_iter_count", Params: []Type{TPtr}, Ret: TI64}
	rtIterAt              = RuntimeFn{Name: "rt_iter_at", Params: []Type{TPtr, TI64, TI32}, Ret: TPtr}
	rtIterAtInt           = RuntimeFn{Name: "rt_iter_at_int", Params: []Type{TPtr, TI64}, Ret: TI64}
	rtBuiltin             = RuntimeFn{Name: "rt_builtin", Params: []Type{TPtr, TI32, TPtr}, Ret: TPtr}
	rtStd                 = RuntimeFn{Name: "rt_std", Params: []Type{TPtr, TI32, TPtr}, Ret: TPtr}
	rtStdId               = RuntimeFn{Name: "rt_std_id", Params: []Type{TI32, TI32, TPtr}, Ret: TPtr}
	rtStdClosure          = RuntimeFn{Name: "rt_std_closure", Params: []Type{TPtr, TI32, TPtr}, Ret: TPtr}
	rtStdBind             = RuntimeFn{Name: "rt_std_bind", Params: []Type{TPtr, TI32, TI32, TPtr}, Ret: TPtr}
	rtCallSlot            = RuntimeFn{Name: "rt_call_slot", Params: []Type{TPtr, TI32, TI32, TPtr, TI32}, Ret: TPtr}
	rtBinop               = RuntimeFn{Name: "rt_binop", Params: []Type{TPtr, TPtr, TPtr}, Ret: TPtr}
	rtUnop                = RuntimeFn{Name: "rt_unop", Params: []Type{TPtr, TPtr}, Ret: TPtr}
	rtCast                = RuntimeFn{Name: "rt_cast", Params: []Type{TPtr, TI32}, Ret: TPtr}
	rtRecord              = RuntimeFn{Name: "rt_record", Params: []Type{TPtr, TI64, TPtr}, Ret: TPtr}
	rtVariant             = RuntimeFn{Name: "rt_variant", Params: []Type{TPtr, TI64, TPtr}, Ret: TPtr}
	rtNullary             = RuntimeFn{Name: "rt_nullary", Params: []Type{TPtr}, Ret: TPtr}
	rtConstVariant        = RuntimeFn{Name: "rt_const_variant", Params: []Type{TPtr, TPtr, TPtr, TI64}, Ret: TVoid}
	rtConstRecord         = RuntimeFn{Name: "rt_const_record", Params: []Type{TPtr, TPtr, TPtr, TI64}, Ret: TVoid}
	rtConstNum            = RuntimeFn{Name: "rt_const_num", Params: []Type{TPtr, TI32, TI32, TI64, TDouble}, Ret: TVoid}
	rtConstInt128         = RuntimeFn{Name: "rt_const_int128", Params: []Type{TPtr, TI32, TI64, TI64}, Ret: TVoid}
	rtBoxInt128           = RuntimeFn{Name: "rt_box_int128", Params: []Type{TI64, TI64, TI32}, Ret: TPtr}
	rtInt128Lo            = RuntimeFn{Name: "rt_int128_lo", Params: []Type{TPtr}, Ret: TI64}
	rtInt128Hi            = RuntimeFn{Name: "rt_int128_hi", Params: []Type{TPtr}, Ret: TI64}
	rtConstText           = RuntimeFn{Name: "rt_const_text", Params: []Type{TPtr, TI32, TPtr, TI64}, Ret: TVoid}
	rtInitNullary         = RuntimeFn{Name: "rt_init_nullary", Params: []Type{TI32, TPtr}, Ret: TVoid}
	rtField               = RuntimeFn{Name: "rt_field", Params: []Type{TPtr, TI64}, Ret: TPtr}
	rtFieldMove           = RuntimeFn{Name: "rt_field_move", Params: []Type{TPtr, TI64}, Ret: TPtr}
	rtSetfield            = RuntimeFn{Name: "rt_setfield", Params: []Type{TPtr, TI64, TPtr}, Ret: TVoid}
	rtArray               = RuntimeFn{Name: "rt_array", Params: []Type{TI64, TPtr}, Ret: TPtr}
	rtIndex               = RuntimeFn{Name: "rt_index", Params: []Type{TPtr, TPtr, TPtr}, Ret: TPtr}
	rtSetindex            = RuntimeFn{Name: "rt_setindex", Params: []Type{TPtr, TPtr, TPtr, TPtr}, Ret: TVoid}
	rtSetindexCopy        = RuntimeFn{Name: "rt_setindex_copy", Params: []Type{TPtr, TPtr, TPtr, TPtr}, Ret: TVoid}
	rtSetfieldCopy        = RuntimeFn{Name: "rt_setfield_copy", Params: []Type{TPtr, TI64, TPtr}, Ret: TVoid}
	rtPayload             = RuntimeFn{Name: "rt_payload", Params: []Type{TPtr, TI64}, Ret: TPtr}
	rtIsvariant           = RuntimeFn{Name: "rt_isvariant", Params: []Type{TPtr, TPtr}, Ret: TPtr}
	rtVariantIndex        = RuntimeFn{Name: "rt_variant_index", Params: []Type{TPtr}, Ret: TI32}
	rtIstype              = RuntimeFn{Name: "rt_istype", Params: []Type{TPtr, TPtr}, Ret: TPtr}
	rtUnbox               = RuntimeFn{Name: "rt_unbox", Params: []Type{TPtr}, Ret: TPtr}
	rtBox                 = RuntimeFn{Name: "rt_box", Params: []Type{TPtr, TPtr, TPtr}, Ret: TPtr}
	rtBoxLent             = RuntimeFn{Name: "rt_box_lent", Params: []Type{TPtr, TPtr, TPtr, TPtr}, Ret: TVoid}
	rtClosure             = RuntimeFn{Name: "rt_closure", Params: []Type{TPtr, TI32, TPtr}, Ret: TPtr}
	rtClosureAsync        = RuntimeFn{Name: "rt_closure_async", Params: []Type{TPtr, TI32, TPtr}, Ret: TPtr}
	rtClosureBindAsync    = RuntimeFn{Name: "rt_closure_bind_async", Params: []Type{TPtr, TI32, TPtr}, Ret: TPtr}
	rtClosureEnv          = RuntimeFn{Name: "rt_closure_env", Params: []Type{TPtr}, Ret: TPtr}
	rtArrayPush           = RuntimeFn{Name: "rt_array_push", Params: []Type{TPtr, TPtr}, Ret: TVoid}
	rtFnItemInit          = RuntimeFn{Name: "rt_fnitem_init", Params: []Type{TPtr, TPtr, TI32}, Ret: TVoid}
	rtFnItemLent          = RuntimeFn{Name: "rt_fnitem_lent", Params: []Type{TPtr, TI32}, Ret: TVoid}
	rtClosureLent         = RuntimeFn{Name: "rt_closure_lent", Params: []Type{TPtr, TI32}, Ret: TPtr}
	rtCallvM              = RuntimeFn{Name: "rt_callv_m", Params: []Type{TPtr, TI32, TPtr, TI32, TI32}, Ret: TPtr}
	rtCallvI64M           = RuntimeFn{Name: "rt_callv_i64_m", Params: []Type{TPtr, TI32, TPtr, TI32, TI32}, Ret: TI64}
	rtStdItemInit         = RuntimeFn{Name: "rt_stditem_init", Params: []Type{TPtr, TPtr, TI32}, Ret: TVoid}
	rtCell                = RuntimeFn{Name: "rt_cell", Params: []Type{TPtr}, Ret: TPtr}
	rtCellInit            = RuntimeFn{Name: "rt_cell_init", Params: []Type{TPtr, TPtr}, Ret: TVoid}
	rtCellget             = RuntimeFn{Name: "rt_cellget", Params: []Type{TPtr}, Ret: TPtr}
	rtCellTake            = RuntimeFn{Name: "rt_cell_take", Params: []Type{TPtr}, Ret: TPtr}
	rtCellset             = RuntimeFn{Name: "rt_cellset", Params: []Type{TPtr, TPtr}, Ret: TVoid}
	rtInterp              = RuntimeFn{Name: "rt_interp", Params: []Type{TI32, TPtr}, Ret: TPtr}
	rtDisplay             = RuntimeFn{Name: "rt_display", Params: []Type{TPtr}, Ret: TPtr}
	rtDrop                = RuntimeFn{Name: "rt_drop", Params: []Type{TPtr}, Ret: TVoid}
	rtDropPartial         = RuntimeFn{Name: "rt_drop_partial", Params: []Type{TPtr}, Ret: TVoid}
	rtDropPartialOuter    = RuntimeFn{Name: "rt_drop_partial_outer", Params: []Type{TPtr}, Ret: TVoid}
	rtReleaseOuterPartial = RuntimeFn{Name: "rt_release_outer_partial", Params: []Type{TPtr}, Ret: TI32}
	rtDropOuter           = RuntimeFn{Name: "rt_drop_outer", Params: []Type{TPtr}, Ret: TVoid}
	rtReleaseOuter        = RuntimeFn{Name: "rt_release_outer", Params: []Type{TPtr}, Ret: TI32}
	rtSweepRelease        = RuntimeFn{Name: "rt_sweep_release", Params: []Type{TPtr}, Ret: TVoid}
	rtOuterFatal          = RuntimeFn{Name: "rt_outer_fatal", Params: []Type{}, Ret: TVoid}
	rtRetain              = RuntimeFn{Name: "rt_retain", Params: []Type{TPtr}, Ret: TPtr}
	rtRelease             = RuntimeFn{Name: "rt_release", Params: []Type{TPtr}, Ret: TVoid}
	rtPanic               = RuntimeFn{Name: "rt_panic", Params: []Type{TPtr}, Ret: TVoid}
	rtPanicIndex          = RuntimeFn{Name: "rt_panic_index", Params: []Type{TI64, TI64, TPtr}, Ret: TVoid}
	rtIntVal              = RuntimeFn{Name: "rt_int_val", Params: []Type{TPtr}, Ret: TI64}
	rtPtr                 = RuntimeFn{Name: "rt_ptr", Params: []Type{TPtr}, Ret: TPtr}
	rtConvert             = RuntimeFn{Name: "rt_convert", Params: []Type{TPtr, TI32, TI32}, Ret: TPtr}
	rtPtrVal              = RuntimeFn{Name: "rt_ptr_val", Params: []Type{TPtr}, Ret: TPtr}
	rtFloatVal            = RuntimeFn{Name: "rt_float_val", Params: []Type{TPtr}, Ret: TDouble}
	rtCharVal             = RuntimeFn{Name: "rt_char_val", Params: []Type{TPtr}, Ret: TI32}
	rtCstr                = RuntimeFn{Name: "rt_cstr", Params: []Type{TPtr}, Ret: TPtr}
	rtFromCstr            = RuntimeFn{Name: "rt_from_cstr", Params: []Type{TPtr}, Ret: TPtr}
	rtErrorAs             = RuntimeFn{Name: "rt_error_as", Params: []Type{TPtr, TPtr, TPtr, TI64}, Ret: TPtr}

	rtFutureNew        = RuntimeFn{Name: "rt_future_new", Params: []Type{TPtr, TI32, TPtr}, Ret: TPtr}
	rtFutureSlots      = RuntimeFn{Name: "rt_future_slots", Params: []Type{TPtr}, Ret: TPtr}
	rtFutureState      = RuntimeFn{Name: "rt_future_state", Params: []Type{TPtr}, Ret: TI32}
	rtFutureArg        = RuntimeFn{Name: "rt_future_arg", Params: []Type{TPtr, TI64}, Ret: TPtr}
	rtFutureStarted    = RuntimeFn{Name: "rt_future_started", Params: []Type{TPtr}, Ret: TVoid}
	rtFutureSuspend    = RuntimeFn{Name: "rt_future_suspend", Params: []Type{TPtr, TI32, TPtr}, Ret: TVoid}
	rtFutureResume     = RuntimeFn{Name: "rt_future_resume", Params: []Type{TPtr}, Ret: TPtr}
	rtFutureComplete   = RuntimeFn{Name: "rt_future_complete", Params: []Type{TPtr, TPtr}, Ret: TPtr}
	rtFutureSettle     = RuntimeFn{Name: "rt_future_settle", Params: []Type{TPtr}, Ret: TPtr}
	rtTaskClose        = RuntimeFn{Name: "rt_task_close", Params: []Type{TPtr}, Ret: TPtr}
	rtUnwindEnter      = RuntimeFn{Name: "rt_unwind_enter", Params: []Type{TPtr, TI64}, Ret: TVoid}
	rtUnwindEnterSlots = RuntimeFn{Name: "rt_unwind_enter_slots", Params: []Type{TPtr, TPtr, TPtr, TI64}, Ret: TVoid}
	rtUnwindEnterStd   = RuntimeFn{Name: "rt_unwind_enter_std", Params: []Type{TPtr, TPtr, TI64, TI64}, Ret: TVoid}
	rtUnwindLeave      = RuntimeFn{Name: "rt_unwind_leave", Params: []Type{TPtr}, Ret: TVoid}

	rtSetfieldSwap     = RuntimeFn{Name: "rt_setfield_swap", Params: []Type{TPtr, TI64, TPtr}, Ret: TPtr}
	rtSetindexSwap     = RuntimeFn{Name: "rt_setindex_swap", Params: []Type{TPtr, TPtr, TPtr, TPtr}, Ret: TPtr}
	rtSetindexCopySwap = RuntimeFn{Name: "rt_setindex_copy_swap", Params: []Type{TPtr, TPtr, TPtr, TPtr}, Ret: TPtr}
	rtSetfieldCopySwap = RuntimeFn{Name: "rt_setfield_copy_swap", Params: []Type{TPtr, TI64, TPtr}, Ret: TPtr}
	rtCellsetSwap      = RuntimeFn{Name: "rt_cellset_swap", Params: []Type{TPtr, TPtr}, Ret: TPtr}
)

// runtimeFns lists every RuntimeFn above in `declare` order: the single
// source ir.RenderExterns and runtime_fns_test.go both read.
var runtimeFns = []RuntimeFn{
	rtInit, rtAbiCheck, rtExitCode, rtUnit, rtInt, rtFloat, rtBool, rtChar, rtStr, rtBytes, rtCopy, rtCopyMut,
	rtTruth, rtAt, rtCallv, rtCallvI64, rtIterCount, rtIterAt, rtIterAtInt, rtBuiltin, rtStd, rtStdId, rtStdClosure, rtStdBind, rtCallSlot,
	rtBinop, rtUnop, rtCast,
	rtRecord, rtVariant, rtNullary, rtConstVariant, rtConstRecord, rtConstNum, rtConstInt128, rtBoxInt128, rtInt128Lo, rtInt128Hi, rtConstText, rtInitNullary, rtField, rtFieldMove, rtSetfield, rtArray,
	rtIndex, rtSetindex, rtSetindexCopy, rtSetfieldCopy, rtPayload, rtIsvariant, rtVariantIndex, rtIstype, rtUnbox, rtBox, rtBoxLent,
	rtClosure, rtClosureAsync, rtClosureBindAsync, rtClosureEnv, rtArrayPush, rtFnItemInit, rtFnItemLent, rtClosureLent, rtCallvM, rtCallvI64M, rtStdItemInit, rtCell, rtCellInit, rtCellget, rtCellTake, rtCellset, rtInterp, rtDisplay, rtDrop,
	rtRetain, rtRelease, rtPanic, rtPanicIndex, rtIntVal, rtPtr, rtConvert, rtPtrVal,
	rtFloatVal, rtCstr, rtFromCstr, rtCharVal, rtErrorAs,
	rtFutureNew, rtFutureSlots, rtFutureState, rtFutureArg, rtFutureStarted,
	rtFutureSuspend, rtFutureResume, rtFutureComplete, rtFutureSettle, rtTaskClose, rtUnwindEnter, rtUnwindEnterSlots, rtUnwindEnterStd, rtUnwindLeave,
	rtDropOuter, rtReleaseOuter, rtSweepRelease, rtOuterFatal, rtDropPartial, rtDropPartialOuter, rtReleaseOuterPartial,
	rtSetfieldSwap, rtSetindexSwap, rtSetindexCopySwap, rtSetfieldCopySwap, rtCellsetSwap,
	rtDLen, rtDArrayLen, rtDMapSeed, rtDBytesGet, rtDArrayGet, rtDParseFloat,
	rtDHashInt, rtDHashBool, rtDHashChar, rtDHashFloat, rtDTry, rtDTryOpt,
}
