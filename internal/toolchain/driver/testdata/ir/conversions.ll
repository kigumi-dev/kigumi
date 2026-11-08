%rt_type = type { ptr, i32, i32, ptr, i32, ptr, ptr, i64, i64 }
%rt_variant = type { ptr, ptr, i32, i32, [64 x i8] }
%"kg.agg.golden.Point" = type { i64, i64 }
%"kg.adt.Option[usize]" = type { i32, [1 x i64] }
%"kg.adt.Option[Int]" = type { i32, [1 x i64] }
%"kg.adt.std/prelude.AllocError" = type { i32, [0 x i8] }
%"kg.adt.Option[u8]" = type { i32, [1 x i64] }
%"kg.adt.Option[i32]" = type { i32, [1 x i64] }
%"kg.adt.Option[String]" = type { i32, [1 x i64] }
%"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]" = type { i32, [1 x i64] }
declare void @llvm.memcpy.p0.p0.i64(ptr, ptr, i64, i1)
declare void @llvm.memset.p0.i64(ptr, i8, i64, i1)
declare { i64, i1 } @llvm.uadd.with.overflow.i64(i64, i64)
declare { i64, i1 } @llvm.usub.with.overflow.i64(i64, i64)
declare void @rt_init(i32, ptr, i32, ptr)
declare void @rt_abi_check(ptr)
declare i32 @rt_exit_code(ptr, ptr, ptr, i64, i64)
declare ptr @rt_unit()
declare ptr @rt_int(i64, i32)
declare ptr @rt_bool(i32)
declare ptr @rt_at(ptr, i64)
declare ptr @rt_builtin(ptr, i32, ptr)
declare ptr @rt_std_id(i32, i32, ptr)
declare ptr @rt_record(ptr, i64, ptr)
declare void @rt_const_num(ptr, i32, i32, i64, double)
declare ptr @rt_field(ptr, i64)
declare ptr @rt_index(ptr, ptr, ptr)
declare ptr @rt_payload(ptr, i64)
declare i32 @rt_variant_index(ptr)
declare void @rt_cell_init(ptr, ptr)
declare ptr @rt_interp(i32, ptr)
declare void @rt_drop(ptr)
declare ptr @rt_retain(ptr)
declare void @rt_release(ptr)
declare void @rt_panic(ptr)
declare i64 @rt_int_val(ptr)
declare i64 @rt_d_array_len(ptr)
declare i64 @rt_d_hash_int(i64, i32)
declare ptr @rt_d_try(i32, i32, ptr)
@".type.universe.Option" = global %rt_type { ptr @.p0, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Option.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Option.Some" = global %rt_variant { ptr @.p1, ptr @".type.universe.Option", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Option.None" = global %rt_variant { ptr @.p2, ptr @".type.universe.Option", i32 1, i32 0, [64 x i8] zeroinitializer }
@".type.universe.Option.variants" = global [2 x ptr] [ptr @".variant.universe.Option.Some", ptr @".variant.universe.Option.None"]
@".type.universe.Result" = global %rt_type { ptr @.p3, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Result.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Result.Ok" = global %rt_variant { ptr @.p4, ptr @".type.universe.Result", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Result.Err" = global %rt_variant { ptr @.p5, ptr @".type.universe.Result", i32 1, i32 1, [64 x i8] zeroinitializer }
@".type.universe.Result.variants" = global [2 x ptr] [ptr @".variant.universe.Result.Ok", ptr @".variant.universe.Result.Err"]
@".type.universe.Array" = global %rt_type { ptr @.p6, i32 0, i32 0, ptr null, i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.golden.Point" = global %rt_type { ptr @.p7, i32 0, i32 2, ptr @".type.golden.Point.fields", i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.golden.Point.fields" = global [2 x ptr] [ptr @.p8, ptr @.p9]
@".type.std/map.Map" = global %rt_type { ptr @.p10, i32 4, i32 5, ptr @".type.std/map.Map.fields", i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.std/map.Map.fields" = global [5 x ptr] [ptr @.p11, ptr @.p12, ptr @.p13, ptr @.p14, ptr @.p15]
@".type.std/prelude.AllocError" = global %rt_type { ptr @.p16, i32 2, i32 0, ptr null, i32 1, ptr @".type.std/prelude.AllocError.variants", ptr null, i64 0, i64 0 }
@".variant.std/prelude.AllocError.OutOfMemory" = global %rt_variant { ptr @.p17, ptr @".type.std/prelude.AllocError", i32 0, i32 0, [64 x i8] zeroinitializer }
@".type.std/prelude.AllocError.variants" = global [1 x ptr] [ptr @".variant.std/prelude.AllocError.OutOfMemory"]
@rt_types = global [6 x ptr] [ptr @".type.universe.Option", ptr @".type.universe.Result", ptr @".type.universe.Array", ptr @".type.golden.Point", ptr @".type.std/map.Map", ptr @".type.std/prelude.AllocError"]
@"golden.Point.equals$bind.fnitem" = internal global [64 x i8] zeroinitializer, align 16
@"golden.Point.hash$bind.fnitem" = internal global [64 x i8] zeroinitializer, align 16
define i1 @"golden.Point.equals"(%"kg.agg.golden.Point" %p0, %"kg.agg.golden.Point" %p1) {
entry:
  %l0 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l0, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l1 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l1, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l2 = alloca i1
  store i1 0, ptr %l2
  store %"kg.agg.golden.Point" %p0, ptr %l0
  store %"kg.agg.golden.Point" %p1, ptr %l1
  br label %b0
b0:
  %v1 = call i1 @"kg.eq.golden.Point"(ptr %l0, ptr %l1)
  store i1 %v1, ptr %l2
  %v2 = load i1, ptr %l2
  ret i1 %v2
}
define i64 @"golden.Point.hash"(%"kg.agg.golden.Point" %p0) {
entry:
  %l0 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l0, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l1 = alloca i64
  store i64 0, ptr %l1
  store %"kg.agg.golden.Point" %p0, ptr %l0
  br label %b0
b0:
  %v1 = call i64 @"kg.hash.golden.Point"(ptr %l0)
  store i64 %v1, ptr %l1
  %v2 = load i64, ptr %l1
  ret i64 %v2
}
define ptr @"kigumi.main"() {
entry:
  %l0 = alloca i64
  store i64 0, ptr %l0
  %l1 = alloca i64
  store i64 0, ptr %l1
  %l2 = alloca i64
  store i64 0, ptr %l2
  %l3 = alloca i64
  store i64 0, ptr %l3
  %l4 = alloca i64
  store i64 0, ptr %l4
  %l5 = alloca i64
  store i64 0, ptr %l5
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l7, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l8 = alloca i64
  store i64 0, ptr %l8
  %l9 = alloca i1
  store i1 0, ptr %l9
  %l10 = alloca i64
  store i64 0, ptr %l10
  %l11 = alloca ptr
  store ptr null, ptr %l11
  %l12 = alloca i64
  store i64 0, ptr %l12
  %l13 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l13, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l14 = alloca i64
  store i64 0, ptr %l14
  %l15 = alloca i1
  store i1 0, ptr %l15
  %l16 = alloca i64
  store i64 0, ptr %l16
  %l17 = alloca ptr
  store ptr null, ptr %l17
  %l18 = alloca i64
  store i64 0, ptr %l18
  %l19 = alloca %"kg.adt.Option[u8]"
  call void @llvm.memset.p0.i64(ptr %l19, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[u8]", ptr null, i32 1) to i64), i1 false)
  %l20 = alloca i64
  store i64 0, ptr %l20
  %l21 = alloca i1
  store i1 0, ptr %l21
  %l22 = alloca i64
  store i64 0, ptr %l22
  %l23 = alloca ptr
  store ptr null, ptr %l23
  %l24 = alloca i64
  store i64 0, ptr %l24
  %l25 = alloca %"kg.adt.Option[u8]"
  call void @llvm.memset.p0.i64(ptr %l25, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[u8]", ptr null, i32 1) to i64), i1 false)
  %l26 = alloca i64
  store i64 0, ptr %l26
  %l27 = alloca i1
  store i1 0, ptr %l27
  %l28 = alloca i64
  store i64 0, ptr %l28
  %l29 = alloca ptr
  store ptr null, ptr %l29
  %l30 = alloca i64
  store i64 0, ptr %l30
  %l31 = alloca %"kg.adt.Option[i32]"
  call void @llvm.memset.p0.i64(ptr %l31, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[i32]", ptr null, i32 1) to i64), i1 false)
  %l32 = alloca i64
  store i64 0, ptr %l32
  %l33 = alloca i1
  store i1 0, ptr %l33
  %l34 = alloca i64
  store i64 0, ptr %l34
  %l35 = alloca ptr
  store ptr null, ptr %l35
  %l36 = alloca i64
  store i64 0, ptr %l36
  %l37 = alloca ptr
  store ptr null, ptr %l37
  %l38 = alloca i64
  store i64 0, ptr %l38
  %l39 = alloca i64
  store i64 0, ptr %l39
  %l40 = alloca i64
  store i64 0, ptr %l40
  %l41 = alloca i64
  store i64 0, ptr %l41
  %l42 = alloca i64
  store i64 0, ptr %l42
  %l43 = alloca i64
  store i64 0, ptr %l43
  %l44 = alloca ptr
  store ptr null, ptr %l44
  %l45 = alloca %"kg.adt.Option[Int]"
  call void @llvm.memset.p0.i64(ptr %l45, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[Int]", ptr null, i32 1) to i64), i1 false)
  %l46 = alloca i64
  store i64 0, ptr %l46
  %l47 = alloca i1
  store i1 0, ptr %l47
  %l48 = alloca i64
  store i64 0, ptr %l48
  %l49 = alloca ptr
  store ptr null, ptr %l49
  %l50 = alloca i64
  store i64 0, ptr %l50
  %l51 = alloca i64
  store i64 0, ptr %l51
  %l52 = alloca i64
  store i64 0, ptr %l52
  %l53 = alloca ptr
  store ptr null, ptr %l53
  %l54 = alloca ptr
  store ptr null, ptr %l54
  %l55 = alloca ptr
  store ptr null, ptr %l55
  %l56 = alloca ptr
  store ptr null, ptr %l56
  %l57 = alloca ptr
  store ptr null, ptr %l57
  %l58 = alloca ptr
  store ptr null, ptr %l58
  %l59 = alloca ptr
  store ptr null, ptr %l59
  %l60 = alloca i64
  store i64 0, ptr %l60
  %l61 = alloca i64
  store i64 0, ptr %l61
  %l62 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l62, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l63 = alloca ptr
  store ptr null, ptr %l63
  %l64 = alloca ptr
  store ptr null, ptr %l64
  %l65 = alloca ptr
  store ptr null, ptr %l65
  %l66 = alloca ptr
  store ptr null, ptr %l66
  %l67 = alloca ptr
  store ptr null, ptr %l67
  %l68 = alloca ptr
  store ptr null, ptr %l68
  %l69 = alloca ptr
  store ptr null, ptr %l69
  %l70 = alloca ptr
  store ptr null, ptr %l70
  %l71 = alloca i64
  store i64 0, ptr %l71
  %l72 = alloca i64
  store i64 0, ptr %l72
  %l73 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l73, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l74 = alloca ptr
  store ptr null, ptr %l74
  %l75 = alloca ptr
  store ptr null, ptr %l75
  %l76 = alloca ptr
  store ptr null, ptr %l76
  %l77 = alloca ptr
  store ptr null, ptr %l77
  %l78 = alloca ptr
  store ptr null, ptr %l78
  %l79 = alloca ptr
  store ptr null, ptr %l79
  %l80 = alloca ptr
  store ptr null, ptr %l80
  %l81 = alloca ptr
  store ptr null, ptr %l81
  %l82 = alloca i64
  store i64 0, ptr %l82
  %l83 = alloca i64
  store i64 0, ptr %l83
  %l84 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l84, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l85 = alloca ptr
  store ptr null, ptr %l85
  %l86 = alloca ptr
  store ptr null, ptr %l86
  %l87 = alloca ptr
  store ptr null, ptr %l87
  %l88 = alloca ptr
  store ptr null, ptr %l88
  %l89 = alloca ptr
  store ptr null, ptr %l89
  %l90 = alloca ptr
  store ptr null, ptr %l90
  %l91 = alloca ptr
  store ptr null, ptr %l91
  %l92 = alloca ptr
  store ptr null, ptr %l92
  %l93 = alloca ptr
  store ptr null, ptr %l93
  %l94 = alloca i64
  store i64 0, ptr %l94
  %l95 = alloca ptr
  store ptr null, ptr %l95
  %l96 = alloca i64
  store i64 0, ptr %l96
  %l97 = alloca i64
  store i64 0, ptr %l97
  %l98 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l98, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l99 = alloca ptr
  store ptr null, ptr %l99
  %l100 = alloca ptr
  store ptr null, ptr %l100
  %l101 = alloca ptr
  store ptr null, ptr %l101
  %l102 = alloca ptr
  store ptr null, ptr %l102
  %l103 = alloca %"kg.adt.Option[String]"
  call void @llvm.memset.p0.i64(ptr %l103, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[String]", ptr null, i32 1) to i64), i1 false)
  %l104 = alloca ptr
  store ptr null, ptr %l104
  %l105 = alloca i1
  store i1 0, ptr %l105
  %l106 = alloca ptr
  store ptr null, ptr %l106
  %l107 = alloca ptr
  store ptr null, ptr %l107
  %l108 = alloca ptr
  store ptr null, ptr %l108
  %l109 = alloca i64
  store i64 0, ptr %l109
  %l110 = alloca i64
  store i64 0, ptr %l110
  %l111 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l111, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l112 = alloca ptr
  store ptr null, ptr %l112
  %l113 = alloca i64
  store i64 0, ptr %l113
  %l114 = alloca i64
  store i64 0, ptr %l114
  %l115 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l115, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l116 = alloca i1
  store i1 0, ptr %l116
  %l117 = alloca ptr
  store ptr null, ptr %l117
  %l118 = alloca ptr
  store ptr null, ptr %l118
  %l119 = alloca ptr
  store ptr null, ptr %l119
  %l120 = alloca ptr
  store ptr null, ptr %l120
  %l121 = alloca ptr
  store ptr null, ptr %l121
  %v1 = alloca i64
  %v2 = alloca i64
  %v3 = alloca i64
  %v4 = alloca i64
  %v5 = alloca i64
  %v6 = alloca [64 x i8], align 16
  %v7 = alloca [64 x i8], align 16
  %v8 = alloca [64 x i8], align 16
  %v9 = alloca [64 x i8], align 16
  %v10 = alloca [64 x i8], align 16
  %v11 = alloca ptr, i32 10
  %v12 = alloca ptr, i32 1
  %v13 = alloca i64
  %v14 = alloca i64
  %v15 = alloca i64
  %v16 = alloca [64 x i8], align 16
  %v17 = alloca [64 x i8], align 16
  %v18 = alloca [64 x i8], align 16
  %v19 = alloca ptr, i32 6
  %v20 = alloca ptr, i32 1
  %v21 = alloca ptr, i32 1
  %v22 = alloca ptr, i32 1
  %v23 = alloca [64 x i8], align 16
  %v24 = alloca ptr, i32 1
  %v25 = alloca ptr, i32 1
  %v26 = alloca ptr, i32 1
  %v27 = alloca ptr, i32 1
  %v28 = alloca [64 x i8], align 16
  %v29 = alloca ptr, i32 1
  %v30 = alloca ptr, i32 1
  %v31 = alloca ptr, i32 1
  %v32 = alloca ptr, i32 1
  %v33 = alloca [64 x i8], align 16
  %v34 = alloca ptr, i32 1
  %v35 = alloca ptr, i32 1
  %v36 = alloca ptr, i32 1
  %v37 = alloca ptr, i32 1
  %v38 = alloca ptr, i32 1
  %v39 = alloca ptr, i32 1
  %v40 = alloca ptr, i32 1
  %v41 = alloca ptr, i32 1
  %v42 = alloca ptr, i32 1
  %v43 = alloca ptr, i32 1
  %v44 = alloca [64 x i8], align 16
  %v45 = alloca ptr, i32 6
  %v46 = alloca ptr, i32 1
  br label %b0
b0:
  store i64 300, ptr %l0
  %v47 = load i64, ptr %l0
  store i64 %v47, ptr %l1
  store i64 -1, ptr %l2
  %v48 = load i64, ptr %l2
  store i64 %v48, ptr %l3
  store i64 200, ptr %l4
  %v49 = load i64, ptr %l4
  store i64 %v49, ptr %l5
  %v50 = load i64, ptr %l3
  %v51 = icmp sge i64 %v50, 0
  %v52 = and i1 true, %v51
  %v53 = select i1 %v52, i64 %v50, i64 0
  store i64 %v53, ptr %v1
  %v54 = zext i1 %v52 to i32
  %v55 = load i64, ptr %v1
  %v56 = icmp ne i32 %v54, 0
  %v57 = select i1 %v56, i32 0, i32 1
  store %"kg.adt.Option[usize]" zeroinitializer, ptr %l7
  %v58 = getelementptr %"kg.adt.Option[usize]", ptr %l7, i32 0, i32 0
  store i32 %v57, ptr %v58
  %v59 = getelementptr %"kg.adt.Option[usize]", ptr %l7, i32 0, i32 1
  store i64 %v55, ptr %v59
  %v60 = getelementptr %"kg.adt.Option[usize]", ptr %l7, i32 0, i32 0
  %v61 = load i32, ptr %v60
  %v62 = icmp eq i32 %v61, 0
  store i1 %v62, ptr %l9
  %v63 = load i1, ptr %l9
  br i1 %v63, label %b1, label %b2
b1:
  %v64 = getelementptr %"kg.adt.Option[usize]", ptr %l7, i32 0, i32 1
  %v65 = load i64, ptr %v64
  store i64 %v65, ptr %l10
  %v66 = load i64, ptr %l10
  store i64 %v66, ptr %l8
  br label %b3
b2:
  %v67 = call ptr @rt_unit()
  store ptr %v67, ptr %l11
  store i64 99, ptr %l12
  %v68 = load i64, ptr %l12
  store i64 %v68, ptr %l8
  br label %b3
b3:
  %v69 = load i64, ptr %l1
  %v70 = icmp sge i64 %v69, 0
  %v71 = and i1 true, %v70
  %v72 = select i1 %v71, i64 %v69, i64 0
  store i64 %v72, ptr %v2
  %v73 = zext i1 %v71 to i32
  %v74 = load i64, ptr %v2
  %v75 = icmp ne i32 %v73, 0
  %v76 = select i1 %v75, i32 0, i32 1
  store %"kg.adt.Option[usize]" zeroinitializer, ptr %l13
  %v77 = getelementptr %"kg.adt.Option[usize]", ptr %l13, i32 0, i32 0
  store i32 %v76, ptr %v77
  %v78 = getelementptr %"kg.adt.Option[usize]", ptr %l13, i32 0, i32 1
  store i64 %v74, ptr %v78
  %v79 = getelementptr %"kg.adt.Option[usize]", ptr %l13, i32 0, i32 0
  %v80 = load i32, ptr %v79
  %v81 = icmp eq i32 %v80, 0
  store i1 %v81, ptr %l15
  %v82 = load i1, ptr %l15
  br i1 %v82, label %b4, label %b5
b4:
  %v83 = getelementptr %"kg.adt.Option[usize]", ptr %l13, i32 0, i32 1
  %v84 = load i64, ptr %v83
  store i64 %v84, ptr %l16
  %v85 = load i64, ptr %l16
  store i64 %v85, ptr %l14
  br label %b6
b5:
  %v86 = call ptr @rt_unit()
  store ptr %v86, ptr %l17
  store i64 99, ptr %l18
  %v87 = load i64, ptr %l18
  store i64 %v87, ptr %l14
  br label %b6
b6:
  %v88 = load i64, ptr %l1
  %v89 = icmp sge i64 %v88, 0
  %v90 = icmp ule i64 %v88, 255
  %v91 = and i1 true, %v89
  %v92 = and i1 %v91, %v90
  %v93 = select i1 %v92, i64 %v88, i64 0
  store i64 %v93, ptr %v3
  %v94 = zext i1 %v92 to i32
  %v95 = load i64, ptr %v3
  %v96 = icmp ne i32 %v94, 0
  %v97 = select i1 %v96, i32 0, i32 1
  store %"kg.adt.Option[u8]" zeroinitializer, ptr %l19
  %v98 = getelementptr %"kg.adt.Option[u8]", ptr %l19, i32 0, i32 0
  store i32 %v97, ptr %v98
  %v99 = getelementptr %"kg.adt.Option[u8]", ptr %l19, i32 0, i32 1
  store i64 %v95, ptr %v99
  %v100 = getelementptr %"kg.adt.Option[u8]", ptr %l19, i32 0, i32 0
  %v101 = load i32, ptr %v100
  %v102 = icmp eq i32 %v101, 0
  store i1 %v102, ptr %l21
  %v103 = load i1, ptr %l21
  br i1 %v103, label %b7, label %b8
b7:
  %v104 = getelementptr %"kg.adt.Option[u8]", ptr %l19, i32 0, i32 1
  %v105 = load i64, ptr %v104
  store i64 %v105, ptr %l22
  %v106 = load i64, ptr %l22
  store i64 %v106, ptr %l20
  br label %b9
b8:
  %v107 = call ptr @rt_unit()
  store ptr %v107, ptr %l23
  store i64 0, ptr %l24
  %v108 = load i64, ptr %l24
  store i64 %v108, ptr %l20
  br label %b9
b9:
  %v109 = load i64, ptr %l5
  %v110 = icmp sge i64 %v109, 0
  %v111 = icmp ule i64 %v109, 255
  %v112 = and i1 true, %v110
  %v113 = and i1 %v112, %v111
  %v114 = select i1 %v113, i64 %v109, i64 0
  store i64 %v114, ptr %v4
  %v115 = zext i1 %v113 to i32
  %v116 = load i64, ptr %v4
  %v117 = icmp ne i32 %v115, 0
  %v118 = select i1 %v117, i32 0, i32 1
  store %"kg.adt.Option[u8]" zeroinitializer, ptr %l25
  %v119 = getelementptr %"kg.adt.Option[u8]", ptr %l25, i32 0, i32 0
  store i32 %v118, ptr %v119
  %v120 = getelementptr %"kg.adt.Option[u8]", ptr %l25, i32 0, i32 1
  store i64 %v116, ptr %v120
  %v121 = getelementptr %"kg.adt.Option[u8]", ptr %l25, i32 0, i32 0
  %v122 = load i32, ptr %v121
  %v123 = icmp eq i32 %v122, 0
  store i1 %v123, ptr %l27
  %v124 = load i1, ptr %l27
  br i1 %v124, label %b10, label %b11
b10:
  %v125 = getelementptr %"kg.adt.Option[u8]", ptr %l25, i32 0, i32 1
  %v126 = load i64, ptr %v125
  store i64 %v126, ptr %l28
  %v127 = load i64, ptr %l28
  store i64 %v127, ptr %l26
  br label %b12
b11:
  %v128 = call ptr @rt_unit()
  store ptr %v128, ptr %l29
  store i64 0, ptr %l30
  %v129 = load i64, ptr %l30
  store i64 %v129, ptr %l26
  br label %b12
b12:
  %v130 = load i64, ptr %l1
  %v131 = icmp sge i64 %v130, -2147483648
  %v132 = icmp sle i64 %v130, 2147483647
  %v133 = and i1 true, %v131
  %v134 = and i1 %v133, %v132
  %v135 = select i1 %v134, i64 %v130, i64 0
  store i64 %v135, ptr %v5
  %v136 = zext i1 %v134 to i32
  %v137 = load i64, ptr %v5
  %v138 = icmp ne i32 %v136, 0
  %v139 = select i1 %v138, i32 0, i32 1
  store %"kg.adt.Option[i32]" zeroinitializer, ptr %l31
  %v140 = getelementptr %"kg.adt.Option[i32]", ptr %l31, i32 0, i32 0
  store i32 %v139, ptr %v140
  %v141 = getelementptr %"kg.adt.Option[i32]", ptr %l31, i32 0, i32 1
  store i64 %v137, ptr %v141
  %v142 = getelementptr %"kg.adt.Option[i32]", ptr %l31, i32 0, i32 0
  %v143 = load i32, ptr %v142
  %v144 = icmp eq i32 %v143, 0
  store i1 %v144, ptr %l33
  %v145 = load i1, ptr %l33
  br i1 %v145, label %b13, label %b14
b13:
  %v146 = getelementptr %"kg.adt.Option[i32]", ptr %l31, i32 0, i32 1
  %v147 = load i64, ptr %v146
  store i64 %v147, ptr %l34
  %v148 = load i64, ptr %l34
  store i64 %v148, ptr %l32
  br label %b15
b14:
  %v149 = call ptr @rt_unit()
  store ptr %v149, ptr %l35
  store i64 0, ptr %l36
  %v150 = load i64, ptr %l36
  store i64 %v150, ptr %l32
  br label %b15
b15:
  %v151 = load i64, ptr %l8
  call void @rt_const_num(ptr %v6, i32 1, i32 64, i64 %v151, double 0.0)
  %v152 = load i64, ptr %l14
  call void @rt_const_num(ptr %v7, i32 1, i32 64, i64 %v152, double 0.0)
  %v153 = load i64, ptr %l20
  call void @rt_const_num(ptr %v8, i32 1, i32 8, i64 %v153, double 0.0)
  %v154 = load i64, ptr %l26
  call void @rt_const_num(ptr %v9, i32 1, i32 8, i64 %v154, double 0.0)
  %v155 = load i64, ptr %l32
  call void @rt_const_num(ptr %v10, i32 1, i32 288, i64 %v155, double 0.0)
  %v156 = getelementptr ptr, ptr %v11, i32 0
  store ptr @.p18, ptr %v156
  %v157 = getelementptr ptr, ptr %v11, i32 1
  store ptr %v6, ptr %v157
  %v158 = getelementptr ptr, ptr %v11, i32 2
  store ptr @.p19, ptr %v158
  %v159 = getelementptr ptr, ptr %v11, i32 3
  store ptr %v7, ptr %v159
  %v160 = getelementptr ptr, ptr %v11, i32 4
  store ptr @.p20, ptr %v160
  %v161 = getelementptr ptr, ptr %v11, i32 5
  store ptr %v8, ptr %v161
  %v162 = getelementptr ptr, ptr %v11, i32 6
  store ptr @.p19, ptr %v162
  %v163 = getelementptr ptr, ptr %v11, i32 7
  store ptr %v9, ptr %v163
  %v164 = getelementptr ptr, ptr %v11, i32 8
  store ptr @.p21, ptr %v164
  %v165 = getelementptr ptr, ptr %v11, i32 9
  store ptr %v10, ptr %v165
  %v166 = call ptr @rt_interp(i32 10, ptr %v11)
  call void @rt_release(ptr %v6)
  call void @rt_release(ptr %v7)
  call void @rt_release(ptr %v8)
  call void @rt_release(ptr %v9)
  call void @rt_release(ptr %v10)
  store ptr %v166, ptr %l6
  %v167 = load ptr, ptr %l6
  %v168 = getelementptr ptr, ptr %v12, i32 0
  store ptr %v167, ptr %v168
  %v169 = call ptr @rt_builtin(ptr @.p22, i32 1, ptr %v12)
  store ptr %v169, ptr %l37
  %v170 = load ptr, ptr %l6
  call void @rt_release(ptr %v170)
  %v171 = load ptr, ptr %l37
  call void @rt_drop(ptr %v171)
  store i64 7, ptr %l38
  %v172 = load i64, ptr %l38
  store i64 %v172, ptr %l39
  store i64 9, ptr %l40
  %v173 = load i64, ptr %l40
  store i64 %v173, ptr %l41
  store i64 -5, ptr %l42
  %v174 = load i64, ptr %l42
  store i64 %v174, ptr %l43
  %v175 = load i64, ptr %l39
  %v176 = icmp sge i64 %v175, 0
  %v177 = icmp sge i64 %v175, -9223372036854775808
  %v178 = icmp sle i64 %v175, 9223372036854775807
  %v179 = and i1 true, %v176
  %v180 = and i1 %v179, %v177
  %v181 = and i1 %v180, %v178
  %v182 = select i1 %v181, i64 %v175, i64 0
  store i64 %v182, ptr %v13
  %v183 = zext i1 %v181 to i32
  %v184 = load i64, ptr %v13
  %v185 = icmp ne i32 %v183, 0
  %v186 = select i1 %v185, i32 0, i32 1
  store %"kg.adt.Option[Int]" zeroinitializer, ptr %l45
  %v187 = getelementptr %"kg.adt.Option[Int]", ptr %l45, i32 0, i32 0
  store i32 %v186, ptr %v187
  %v188 = getelementptr %"kg.adt.Option[Int]", ptr %l45, i32 0, i32 1
  store i64 %v184, ptr %v188
  %v189 = getelementptr %"kg.adt.Option[Int]", ptr %l45, i32 0, i32 0
  %v190 = load i32, ptr %v189
  %v191 = icmp eq i32 %v190, 0
  store i1 %v191, ptr %l47
  %v192 = load i1, ptr %l47
  br i1 %v192, label %b16, label %b17
b16:
  %v193 = getelementptr %"kg.adt.Option[Int]", ptr %l45, i32 0, i32 1
  %v194 = load i64, ptr %v193
  store i64 %v194, ptr %l48
  %v195 = load i64, ptr %l48
  store i64 %v195, ptr %l46
  br label %b18
b17:
  %v196 = call ptr @rt_unit()
  store ptr %v196, ptr %l49
  store i64 -1, ptr %l50
  %v197 = load i64, ptr %l50
  store i64 %v197, ptr %l46
  br label %b18
b18:
  %v198 = load i64, ptr %l41
  store i64 %v198, ptr %v14
  %v199 = zext i1 true to i32
  %v200 = load i64, ptr %v14
  store i64 %v200, ptr %l51
  %v201 = load i64, ptr %l43
  store i64 %v201, ptr %v15
  %v202 = zext i1 true to i32
  %v203 = load i64, ptr %v15
  store i64 %v203, ptr %l52
  %v204 = load i64, ptr %l46
  call void @rt_const_num(ptr %v16, i32 1, i32 320, i64 %v204, double 0.0)
  %v205 = load i64, ptr %l51
  call void @rt_const_num(ptr %v17, i32 1, i32 320, i64 %v205, double 0.0)
  %v206 = load i64, ptr %l52
  call void @rt_const_num(ptr %v18, i32 1, i32 320, i64 %v206, double 0.0)
  %v207 = getelementptr ptr, ptr %v19, i32 0
  store ptr @.p23, ptr %v207
  %v208 = getelementptr ptr, ptr %v19, i32 1
  store ptr %v16, ptr %v208
  %v209 = getelementptr ptr, ptr %v19, i32 2
  store ptr @.p19, ptr %v209
  %v210 = getelementptr ptr, ptr %v19, i32 3
  store ptr %v17, ptr %v210
  %v211 = getelementptr ptr, ptr %v19, i32 4
  store ptr @.p19, ptr %v211
  %v212 = getelementptr ptr, ptr %v19, i32 5
  store ptr %v18, ptr %v212
  %v213 = call ptr @rt_interp(i32 6, ptr %v19)
  call void @rt_release(ptr %v16)
  call void @rt_release(ptr %v17)
  call void @rt_release(ptr %v18)
  store ptr %v213, ptr %l44
  %v214 = load ptr, ptr %l44
  %v215 = getelementptr ptr, ptr %v20, i32 0
  store ptr %v214, ptr %v215
  %v216 = call ptr @rt_builtin(ptr @.p22, i32 1, ptr %v20)
  store ptr %v216, ptr %l53
  %v217 = load ptr, ptr %l44
  call void @rt_release(ptr %v217)
  %v218 = load ptr, ptr %l53
  call void @rt_drop(ptr %v218)
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l54
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l55
  %v219 = load ptr, ptr %l54
  %v220 = load ptr, ptr %l55
  %v221 = call ptr @"std/map.empty[golden.Point,String]"(ptr %v219, ptr %v220)
  store ptr %v221, ptr %l56
  %v222 = load ptr, ptr %l56
  store ptr %v222, ptr %l57
  %v223 = load ptr, ptr %l57
  store ptr %v223, ptr %l58
  %v224 = load ptr, ptr %l58
  call void @rt_cell_init(ptr %v23, ptr %v224)
  store ptr %v23, ptr %l59
  store i64 1, ptr %l60
  store i64 2, ptr %l61
  %v225 = getelementptr %"kg.agg.golden.Point", ptr %l62, i32 0, i32 0
  %v226 = load i64, ptr %l60
  store i64 %v226, ptr %v225
  %v227 = getelementptr %"kg.agg.golden.Point", ptr %l62, i32 0, i32 1
  %v228 = load i64, ptr %l61
  store i64 %v228, ptr %v227
  store ptr @.p24, ptr %l63
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l64
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l65
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l66
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l67
  %v229 = call ptr @"kg.box.golden.Point"(ptr %l62)
  store ptr %v229, ptr %l119
  %v230 = load ptr, ptr %l59
  %v231 = load ptr, ptr %l119
  %v232 = load ptr, ptr %l63
  %v233 = load ptr, ptr %l64
  %v234 = load ptr, ptr %l65
  %v235 = load ptr, ptr %l66
  %v236 = load ptr, ptr %l67
  %v237 = call ptr @"std/map.Map.set"(ptr %v230, ptr %v231, ptr %v232, ptr %v233, ptr %v234, ptr %v235, ptr %v236)
  store ptr %v237, ptr %l68
  %v238 = load ptr, ptr %l59
  %v239 = load ptr, ptr %l68
  call void @rt_drop(ptr %v239)
  %v240 = load ptr, ptr %l57
  store ptr %v240, ptr %l69
  %v241 = load ptr, ptr %l69
  call void @rt_cell_init(ptr %v28, ptr %v241)
  store ptr %v28, ptr %l70
  store i64 1, ptr %l71
  store i64 2, ptr %l72
  %v242 = getelementptr %"kg.agg.golden.Point", ptr %l73, i32 0, i32 0
  %v243 = load i64, ptr %l71
  store i64 %v243, ptr %v242
  %v244 = getelementptr %"kg.agg.golden.Point", ptr %l73, i32 0, i32 1
  %v245 = load i64, ptr %l72
  store i64 %v245, ptr %v244
  store ptr @.p25, ptr %l74
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l75
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l76
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l77
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l78
  %v246 = call ptr @"kg.box.golden.Point"(ptr %l73)
  store ptr %v246, ptr %l120
  %v247 = load ptr, ptr %l70
  %v248 = load ptr, ptr %l120
  %v249 = load ptr, ptr %l74
  %v250 = load ptr, ptr %l75
  %v251 = load ptr, ptr %l76
  %v252 = load ptr, ptr %l77
  %v253 = load ptr, ptr %l78
  %v254 = call ptr @"std/map.Map.set"(ptr %v247, ptr %v248, ptr %v249, ptr %v250, ptr %v251, ptr %v252, ptr %v253)
  store ptr %v254, ptr %l79
  %v255 = load ptr, ptr %l70
  %v256 = load ptr, ptr %l79
  call void @rt_drop(ptr %v256)
  %v257 = load ptr, ptr %l57
  store ptr %v257, ptr %l80
  %v258 = load ptr, ptr %l80
  call void @rt_cell_init(ptr %v33, ptr %v258)
  store ptr %v33, ptr %l81
  store i64 3, ptr %l82
  store i64 4, ptr %l83
  %v259 = getelementptr %"kg.agg.golden.Point", ptr %l84, i32 0, i32 0
  %v260 = load i64, ptr %l82
  store i64 %v260, ptr %v259
  %v261 = getelementptr %"kg.agg.golden.Point", ptr %l84, i32 0, i32 1
  %v262 = load i64, ptr %l83
  store i64 %v262, ptr %v261
  store ptr @.p26, ptr %l85
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l86
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l87
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l88
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l89
  %v263 = call ptr @"kg.box.golden.Point"(ptr %l84)
  store ptr %v263, ptr %l121
  %v264 = load ptr, ptr %l81
  %v265 = load ptr, ptr %l121
  %v266 = load ptr, ptr %l85
  %v267 = load ptr, ptr %l86
  %v268 = load ptr, ptr %l87
  %v269 = load ptr, ptr %l88
  %v270 = load ptr, ptr %l89
  %v271 = call ptr @"std/map.Map.set"(ptr %v264, ptr %v265, ptr %v266, ptr %v267, ptr %v268, ptr %v269, ptr %v270)
  store ptr %v271, ptr %l90
  %v272 = load ptr, ptr %l81
  %v273 = load ptr, ptr %l90
  call void @rt_drop(ptr %v273)
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l92
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l93
  %v274 = load ptr, ptr %l57
  %v275 = load ptr, ptr %l92
  %v276 = load ptr, ptr %l93
  %v277 = call i64 @"std/map.Map.len"(ptr %v274, ptr %v275, ptr %v276)
  store i64 %v277, ptr %l94
  store i64 1, ptr %l96
  store i64 2, ptr %l97
  %v278 = getelementptr %"kg.agg.golden.Point", ptr %l98, i32 0, i32 0
  %v279 = load i64, ptr %l96
  store i64 %v279, ptr %v278
  %v280 = getelementptr %"kg.agg.golden.Point", ptr %l98, i32 0, i32 1
  %v281 = load i64, ptr %l97
  store i64 %v281, ptr %v280
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l99
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l100
  store ptr @"golden.Point.equals$bind.fnitem", ptr %l101
  store ptr @"golden.Point.hash$bind.fnitem", ptr %l102
  %v282 = load ptr, ptr %l57
  %v283 = load %"kg.agg.golden.Point", ptr %l98
  %v284 = load ptr, ptr %l99
  %v285 = load ptr, ptr %l100
  %v286 = load ptr, ptr %l101
  %v287 = load ptr, ptr %l102
  %v288 = call %"kg.adt.Option[String]" @"std/map.Map.get[golden.Point,String]"(ptr %v282, %"kg.agg.golden.Point" %v283, ptr %v284, ptr %v285, ptr %v286, ptr %v287)
  store %"kg.adt.Option[String]" %v288, ptr %l103
  %v289 = getelementptr %"kg.adt.Option[String]", ptr %l103, i32 0, i32 0
  %v290 = load i32, ptr %v289
  %v291 = icmp eq i32 %v290, 0
  store i1 %v291, ptr %l105
  %v292 = load i1, ptr %l105
  br i1 %v292, label %b19, label %b20
b19:
  %v293 = getelementptr %"kg.adt.Option[String]", ptr %l103, i32 0, i32 1
  %v294 = load i64, ptr %v293
  %v295 = inttoptr i64 %v294 to ptr
  %v296 = call ptr @rt_retain(ptr %v295)
  store ptr %v296, ptr %l106
  %v297 = load ptr, ptr %l106
  store ptr %v297, ptr %l104
  br label %b21
b20:
  %v298 = call ptr @rt_unit()
  store ptr %v298, ptr %l107
  store ptr @.p27, ptr %l108
  %v299 = load ptr, ptr %l108
  store ptr %v299, ptr %l104
  br label %b21
b21:
  call void @"kg.drop.Option[String]"(ptr %l103)
  store i64 1, ptr %l109
  store i64 2, ptr %l110
  %v300 = getelementptr %"kg.agg.golden.Point", ptr %l111, i32 0, i32 0
  %v301 = load i64, ptr %l109
  store i64 %v301, ptr %v300
  %v302 = getelementptr %"kg.agg.golden.Point", ptr %l111, i32 0, i32 1
  %v303 = load i64, ptr %l110
  store i64 %v303, ptr %v302
  store i64 1, ptr %l113
  store i64 2, ptr %l114
  %v304 = getelementptr %"kg.agg.golden.Point", ptr %l115, i32 0, i32 0
  %v305 = load i64, ptr %l113
  store i64 %v305, ptr %v304
  %v306 = getelementptr %"kg.agg.golden.Point", ptr %l115, i32 0, i32 1
  %v307 = load i64, ptr %l114
  store i64 %v307, ptr %v306
  %v308 = call i1 @"kg.eq.golden.Point"(ptr %l111, ptr %l115)
  store i1 %v308, ptr %l116
  %v309 = load i64, ptr %l94
  call void @rt_const_num(ptr %v44, i32 1, i32 64, i64 %v309, double 0.0)
  %v310 = load ptr, ptr %l104
  %v311 = load i1, ptr %l116
  %v312 = zext i1 %v311 to i32
  %v313 = call ptr @rt_bool(i32 %v312)
  %v314 = getelementptr ptr, ptr %v45, i32 0
  store ptr @.p28, ptr %v314
  %v315 = getelementptr ptr, ptr %v45, i32 1
  store ptr %v44, ptr %v315
  %v316 = getelementptr ptr, ptr %v45, i32 2
  store ptr @.p29, ptr %v316
  %v317 = getelementptr ptr, ptr %v45, i32 3
  store ptr %v310, ptr %v317
  %v318 = getelementptr ptr, ptr %v45, i32 4
  store ptr @.p30, ptr %v318
  %v319 = getelementptr ptr, ptr %v45, i32 5
  store ptr %v313, ptr %v319
  %v320 = call ptr @rt_interp(i32 6, ptr %v45)
  call void @rt_release(ptr %v44)
  store ptr %v320, ptr %l91
  %v321 = load ptr, ptr %l104
  call void @rt_release(ptr %v321)
  %v322 = load ptr, ptr %l91
  %v323 = getelementptr ptr, ptr %v46, i32 0
  store ptr %v322, ptr %v323
  %v324 = call ptr @rt_builtin(ptr @.p22, i32 1, ptr %v46)
  store ptr %v324, ptr %l117
  %v325 = load ptr, ptr %l91
  call void @rt_release(ptr %v325)
  %v326 = load ptr, ptr %l117
  call void @rt_drop(ptr %v326)
  %v327 = load ptr, ptr %l57
  store ptr null, ptr %l57
  call void @rt_drop(ptr %v327)
  %v328 = call ptr @rt_unit()
  store ptr %v328, ptr %l118
  %v329 = load ptr, ptr %l118
  ret ptr %v329
}
define ptr @"std/map.empty[golden.Point,String]"(ptr %p0, ptr %p1) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = alloca ptr
  store ptr null, ptr %l3
  %l4 = alloca %"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]"
  call void @llvm.memset.p0.i64(ptr %l4, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr null, i32 1) to i64), i1 false)
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %l6 = alloca i1
  store i1 0, ptr %l6
  %l7 = alloca ptr
  store ptr null, ptr %l7
  %l8 = alloca %"kg.adt.std/prelude.AllocError"
  call void @llvm.memset.p0.i64(ptr %l8, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.std/prelude.AllocError", ptr null, i32 1) to i64), i1 false)
  %l9 = alloca ptr
  store ptr null, ptr %l9
  %l10 = alloca ptr
  store ptr null, ptr %l10
  %v1 = alloca ptr, i32 2
  %v2 = alloca ptr, i32 1
  store ptr %p0, ptr %l0
  store ptr %p1, ptr %l1
  br label %b0
b0:
  %v3 = load ptr, ptr %l0
  store ptr %v3, ptr %l2
  %v4 = load ptr, ptr %l1
  store ptr %v4, ptr %l3
  %v5 = load ptr, ptr %l2
  %v6 = load ptr, ptr %l3
  %v7 = getelementptr ptr, ptr %v1, i32 0
  store ptr %v5, ptr %v7
  %v8 = getelementptr ptr, ptr %v1, i32 1
  store ptr %v6, ptr %v8
  %v9 = call ptr @rt_d_try(i32 126, i32 2, ptr %v1)
  call void @llvm.memset.p0.i64(ptr %l4, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr null, i32 1) to i64), i1 false)
  %v10 = icmp ne ptr %v9, null
  %v11 = select i1 %v10, i32 0, i32 1
  %v12 = getelementptr %"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr %l4, i32 0, i32 0
  store i32 %v11, ptr %v12
  %v13 = getelementptr %"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr %l4, i32 0, i32 1
  %v14 = ptrtoint ptr %v9 to i64
  store i64 %v14, ptr %v13
  %v15 = getelementptr %"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr %l4, i32 0, i32 0
  %v16 = load i32, ptr %v15
  %v17 = icmp eq i32 %v16, 0
  store i1 %v17, ptr %l6
  %v18 = load i1, ptr %l6
  br i1 %v18, label %b1, label %b2
b1:
  %v19 = getelementptr %"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr %l4, i32 0, i32 1
  %v20 = load i64, ptr %v19
  %v21 = inttoptr i64 %v20 to ptr
  %v22 = call ptr @rt_retain(ptr %v21)
  store ptr %v22, ptr %l7
  %v23 = load ptr, ptr %l7
  store ptr %v23, ptr %l5
  br label %b3
b2:
  %v24 = getelementptr %"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr %l4, i32 0, i32 1
  call void @llvm.memcpy.p0.p0.i64(ptr %l8, ptr %v24, i64 ptrtoint (ptr getelementptr (%"kg.adt.std/prelude.AllocError", ptr null, i32 1) to i64), i1 false)
  store ptr @.p31, ptr %l9
  %v25 = load ptr, ptr %l9
  %v26 = getelementptr ptr, ptr %v2, i32 0
  store ptr %v25, ptr %v26
  %v27 = call ptr @rt_builtin(ptr @.p32, i32 1, ptr %v2)
  store ptr %v27, ptr %l10
  %v28 = load ptr, ptr %l9
  call void @rt_release(ptr %v28)
  %v29 = load ptr, ptr %l10
  call void @rt_release(ptr %v29)
  br label %b3
b3:
  call void @"kg.drop.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]"(ptr %l4)
  %v30 = load ptr, ptr %l1
  store ptr null, ptr %l1
  call void @rt_drop(ptr %v30)
  %v31 = load ptr, ptr %l0
  store ptr null, ptr %l0
  call void @rt_drop(ptr %v31)
  %v32 = load ptr, ptr %l5
  ret ptr %v32
}
define %"kg.adt.Option[String]" @"std/map.Map.get[golden.Point,String]"(ptr %p0, %"kg.agg.golden.Point" %p1, ptr %p2, ptr %p3, ptr %p4, ptr %p5) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l1, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = alloca ptr
  store ptr null, ptr %l3
  %l4 = alloca ptr
  store ptr null, ptr %l4
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %l6 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l6, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l7 = alloca ptr
  store ptr null, ptr %l7
  %l8 = alloca ptr
  store ptr null, ptr %l8
  %l9 = alloca ptr
  store ptr null, ptr %l9
  %l10 = alloca ptr
  store ptr null, ptr %l10
  %l11 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l11, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l12 = alloca i1
  store i1 0, ptr %l12
  %l13 = alloca i64
  store i64 0, ptr %l13
  %l14 = alloca i64
  store i64 0, ptr %l14
  %l15 = alloca ptr
  store ptr null, ptr %l15
  %l16 = alloca ptr
  store ptr null, ptr %l16
  %l17 = alloca ptr
  store ptr null, ptr %l17
  %l18 = alloca %"kg.adt.Option[String]"
  call void @llvm.memset.p0.i64(ptr %l18, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[String]", ptr null, i32 1) to i64), i1 false)
  %l19 = alloca ptr
  store ptr null, ptr %l19
  %l20 = alloca ptr
  store ptr null, ptr %l20
  %l21 = alloca %"kg.adt.Option[String]"
  call void @llvm.memset.p0.i64(ptr %l21, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[String]", ptr null, i32 1) to i64), i1 false)
  %l22 = alloca ptr
  store ptr null, ptr %l22
  %l23 = alloca ptr
  store ptr null, ptr %l23
  %l24 = alloca ptr
  store ptr null, ptr %l24
  %l25 = alloca ptr
  store ptr null, ptr %l25
  %l26 = alloca ptr
  store ptr null, ptr %l26
  %v1 = alloca ptr, i32 2
  store ptr %p0, ptr %l0
  store %"kg.agg.golden.Point" %p1, ptr %l1
  store ptr %p2, ptr %l2
  store ptr %p3, ptr %l3
  store ptr %p4, ptr %l4
  store ptr %p5, ptr %l5
  br label %b0
b0:
  call void @llvm.memcpy.p0.p0.i64(ptr %l6, ptr %l1, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %v2 = load ptr, ptr %l2
  store ptr %v2, ptr %l7
  %v3 = load ptr, ptr %l3
  store ptr %v3, ptr %l8
  %v4 = load ptr, ptr %l2
  store ptr %v4, ptr %l9
  %v5 = load ptr, ptr %l3
  store ptr %v5, ptr %l10
  %v6 = load ptr, ptr %l7
  %v7 = call ptr @rt_retain(ptr %v6)
  store ptr %v7, ptr %l22
  %v8 = load ptr, ptr %l8
  %v9 = call ptr @rt_retain(ptr %v8)
  store ptr %v9, ptr %l23
  %v10 = load ptr, ptr %l9
  %v11 = call ptr @rt_retain(ptr %v10)
  store ptr %v11, ptr %l24
  %v12 = load ptr, ptr %l10
  %v13 = call ptr @rt_retain(ptr %v12)
  store ptr %v13, ptr %l25
  %v14 = load ptr, ptr %l0
  %v15 = load %"kg.agg.golden.Point", ptr %l6
  %v16 = load ptr, ptr %l22
  %v17 = load ptr, ptr %l23
  %v18 = load ptr, ptr %l24
  %v19 = load ptr, ptr %l25
  %v20 = call %"kg.adt.Option[usize]" @"std/map.Map.position[golden.Point,String]"(ptr %v14, %"kg.agg.golden.Point" %v15, ptr %v16, ptr %v17, ptr %v18, ptr %v19)
  store %"kg.adt.Option[usize]" %v20, ptr %l11
  %v21 = getelementptr %"kg.adt.Option[usize]", ptr %l11, i32 0, i32 0
  %v22 = load i32, ptr %v21
  %v23 = icmp eq i32 %v22, 0
  store i1 %v23, ptr %l12
  %v24 = load i1, ptr %l12
  br i1 %v24, label %b1, label %b2
b1:
  %v25 = getelementptr %"kg.adt.Option[usize]", ptr %l11, i32 0, i32 1
  %v26 = load i64, ptr %v25
  store i64 %v26, ptr %l13
  %v27 = load i64, ptr %l13
  store i64 %v27, ptr %l14
  br label %b2
b2:
  %v28 = load i1, ptr %l12
  br i1 %v28, label %b3, label %b4
b3:
  %v29 = load ptr, ptr %l0
  %v30 = call ptr @rt_field(ptr %v29, i64 1)
  store ptr %v30, ptr %l16
  %v31 = load i64, ptr %l14
  %v32 = call ptr @rt_int(i64 %v31, i32 64)
  store ptr %v32, ptr %l17
  %v33 = load ptr, ptr %l16
  %v34 = load ptr, ptr %l17
  %v35 = getelementptr ptr, ptr %v1, i32 0
  store ptr %v33, ptr %v35
  %v36 = getelementptr ptr, ptr %v1, i32 1
  store ptr %v34, ptr %v36
  %v37 = call ptr @rt_std_id(i32 21, i32 2, ptr %v1)
  store ptr %v37, ptr %l26
  call void @rt_release(ptr %v34)
  %v38 = load ptr, ptr %l26
  call void @"kg.unbox.Option[String]"(ptr %l18, ptr %v38)
  call void @rt_release(ptr %v38)
  %v39 = load ptr, ptr %l16
  call void @rt_release(ptr %v39)
  %v40 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v40)
  %v41 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v41)
  %v42 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v42)
  %v43 = load ptr, ptr %l2
  store ptr null, ptr %l2
  call void @rt_drop(ptr %v43)
  %v44 = load %"kg.adt.Option[String]", ptr %l18
  ret %"kg.adt.Option[String]" %v44
b4:
  %v45 = call ptr @rt_unit()
  store ptr %v45, ptr %l20
  %v46 = load ptr, ptr %l20
  store ptr %v46, ptr %l15
  br label %b5
b5:
  %v47 = load ptr, ptr %l15
  call void @rt_drop(ptr %v47)
  call void @llvm.memset.p0.i64(ptr %l21, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[String]", ptr null, i32 1) to i64), i1 false)
  %v48 = getelementptr %"kg.adt.Option[String]", ptr %l21, i32 0, i32 0
  store i32 1, ptr %v48
  %v49 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v49)
  %v50 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v50)
  %v51 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v51)
  %v52 = load ptr, ptr %l2
  store ptr null, ptr %l2
  call void @rt_drop(ptr %v52)
  %v53 = load %"kg.adt.Option[String]", ptr %l21
  ret %"kg.adt.Option[String]" %v53
b6:
  %v54 = call ptr @rt_unit()
  store ptr %v54, ptr %l19
  %v55 = load ptr, ptr %l19
  call void @rt_release(ptr %v55)
  br label %b5
}
define i64 @"std/prelude.hash[golden.Point]"(%"kg.agg.golden.Point" %p0, ptr %p1) {
entry:
  %l0 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l0, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = alloca i64
  store i64 0, ptr %l2
  store %"kg.agg.golden.Point" %p0, ptr %l0
  store ptr %p1, ptr %l1
  br label %b0
b0:
  %v1 = call i64 @"kg.hash.golden.Point"(ptr %l0)
  store i64 %v1, ptr %l2
  %v2 = load ptr, ptr %l1
  store ptr null, ptr %l1
  call void @rt_drop(ptr %v2)
  %v3 = load i64, ptr %l2
  ret i64 %v3
}
define %"kg.adt.Option[usize]" @"std/map.Map.positionHashed[golden.Point,String]"(ptr %p0, i64 %p1, %"kg.agg.golden.Point" %p2, ptr %p3, ptr %p4, ptr %p5, ptr %p6) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca i64
  store i64 0, ptr %l1
  %l2 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l2, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l3 = alloca ptr
  store ptr null, ptr %l3
  %l4 = alloca ptr
  store ptr null, ptr %l4
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = alloca ptr
  store ptr null, ptr %l7
  %l8 = alloca i64
  store i64 0, ptr %l8
  %l9 = alloca i64
  store i64 0, ptr %l9
  %l10 = alloca i1
  store i1 0, ptr %l10
  %l11 = alloca ptr
  store ptr null, ptr %l11
  %l12 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l12, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l13 = alloca ptr
  store ptr null, ptr %l13
  %l14 = alloca ptr
  store ptr null, ptr %l14
  %l15 = alloca i64
  store i64 0, ptr %l15
  %l16 = alloca ptr
  store ptr null, ptr %l16
  %l17 = alloca i64
  store i64 0, ptr %l17
  %l18 = alloca i64
  store i64 0, ptr %l18
  %l19 = alloca i64
  store i64 0, ptr %l19
  %l20 = alloca i64
  store i64 0, ptr %l20
  %l21 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l21, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l22 = alloca ptr
  store ptr null, ptr %l22
  %l23 = alloca ptr
  store ptr null, ptr %l23
  %l24 = alloca ptr
  store ptr null, ptr %l24
  %l25 = alloca ptr
  store ptr null, ptr %l25
  %l26 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l26, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l27 = alloca i1
  store i1 0, ptr %l27
  %l28 = alloca i64
  store i64 0, ptr %l28
  %l29 = alloca i64
  store i64 0, ptr %l29
  %l30 = alloca ptr
  store ptr null, ptr %l30
  %l31 = alloca ptr
  store ptr null, ptr %l31
  %l32 = alloca ptr
  store ptr null, ptr %l32
  %l33 = alloca ptr
  store ptr null, ptr %l33
  %l34 = alloca ptr
  store ptr null, ptr %l34
  %l35 = alloca i64
  store i64 0, ptr %l35
  %l36 = alloca ptr
  store ptr null, ptr %l36
  %l37 = alloca ptr
  store ptr null, ptr %l37
  %l38 = alloca i64
  store i64 0, ptr %l38
  %l39 = alloca i64
  store i64 0, ptr %l39
  %l40 = alloca i64
  store i64 0, ptr %l40
  %l41 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l41, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l42 = alloca ptr
  store ptr null, ptr %l42
  %l43 = alloca ptr
  store ptr null, ptr %l43
  %l44 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l44, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l45 = alloca i64
  store i64 0, ptr %l45
  %l46 = alloca ptr
  store ptr null, ptr %l46
  %l47 = alloca ptr
  store ptr null, ptr %l47
  %l48 = alloca ptr
  store ptr null, ptr %l48
  %l49 = alloca ptr
  store ptr null, ptr %l49
  %l50 = alloca ptr
  store ptr null, ptr %l50
  store ptr %p0, ptr %l0
  store i64 %p1, ptr %l1
  store %"kg.agg.golden.Point" %p2, ptr %l2
  store ptr %p3, ptr %l3
  store ptr %p4, ptr %l4
  store ptr %p5, ptr %l5
  store ptr %p6, ptr %l6
  br label %b0
b0:
  %v1 = load ptr, ptr %l0
  %v2 = call ptr @rt_field(ptr %v1, i64 2)
  store ptr %v2, ptr %l7
  %v3 = load ptr, ptr %l7
  %v4 = call i64 @rt_d_array_len(ptr %v3)
  store i64 %v4, ptr %l8
  %v5 = load ptr, ptr %l7
  call void @rt_release(ptr %v5)
  store i64 0, ptr %l9
  %v6 = load i64, ptr %l8
  %v7 = load i64, ptr %l9
  %v8 = icmp eq i64 %v6, %v7
  store i1 %v8, ptr %l10
  %v9 = load i1, ptr %l10
  br i1 %v9, label %b1, label %b2
b1:
  call void @llvm.memset.p0.i64(ptr %l12, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %v10 = getelementptr %"kg.adt.Option[usize]", ptr %l12, i32 0, i32 0
  store i32 1, ptr %v10
  %v11 = load ptr, ptr %l6
  store ptr null, ptr %l6
  call void @rt_drop(ptr %v11)
  %v12 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v12)
  %v13 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v13)
  %v14 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v14)
  %v15 = load %"kg.adt.Option[usize]", ptr %l12
  ret %"kg.adt.Option[usize]" %v15
b2:
  %v16 = call ptr @rt_unit()
  store ptr %v16, ptr %l14
  %v17 = load ptr, ptr %l14
  store ptr %v17, ptr %l11
  br label %b3
b3:
  %v18 = load ptr, ptr %l11
  call void @rt_drop(ptr %v18)
  %v19 = load i64, ptr %l1
  store i64 %v19, ptr %l15
  %v20 = load ptr, ptr %l0
  %v21 = call ptr @rt_field(ptr %v20, i64 2)
  store ptr %v21, ptr %l16
  %v22 = load ptr, ptr %l16
  %v23 = call i64 @rt_d_array_len(ptr %v22)
  store i64 %v23, ptr %l17
  %v24 = load ptr, ptr %l16
  call void @rt_release(ptr %v24)
  %v25 = load i64, ptr %l15
  store i64 %v25, ptr %l45
  %v26 = load i64, ptr %l45
  %v27 = load i64, ptr %l17
  %v28 = call i64 @"std/map.startSlot"(i64 %v26, i64 %v27)
  store i64 %v28, ptr %l18
  %v29 = load i64, ptr %l18
  store i64 %v29, ptr %l19
  %v30 = load i64, ptr %l19
  store i64 %v30, ptr %l20
  call void @llvm.memcpy.p0.p0.i64(ptr %l21, ptr %l2, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %v31 = load ptr, ptr %l3
  store ptr %v31, ptr %l22
  %v32 = load ptr, ptr %l4
  store ptr %v32, ptr %l23
  %v33 = load ptr, ptr %l3
  store ptr %v33, ptr %l24
  %v34 = load ptr, ptr %l4
  store ptr %v34, ptr %l25
  %v35 = load i64, ptr %l20
  %v36 = call ptr @rt_int(i64 %v35, i32 64)
  store ptr %v36, ptr %l46
  %v37 = load ptr, ptr %l22
  %v38 = call ptr @rt_retain(ptr %v37)
  store ptr %v38, ptr %l47
  %v39 = load ptr, ptr %l23
  %v40 = call ptr @rt_retain(ptr %v39)
  store ptr %v40, ptr %l48
  %v41 = load ptr, ptr %l24
  %v42 = call ptr @rt_retain(ptr %v41)
  store ptr %v42, ptr %l49
  %v43 = load ptr, ptr %l25
  %v44 = call ptr @rt_retain(ptr %v43)
  store ptr %v44, ptr %l50
  %v45 = load ptr, ptr %l0
  %v46 = load ptr, ptr %l46
  %v47 = load %"kg.agg.golden.Point", ptr %l21
  %v48 = load ptr, ptr %l47
  %v49 = load ptr, ptr %l48
  %v50 = load ptr, ptr %l49
  %v51 = load ptr, ptr %l50
  %v52 = call %"kg.adt.Option[usize]" @"std/map.Map.findSlot[golden.Point,String]"(ptr %v45, ptr %v46, %"kg.agg.golden.Point" %v47, ptr %v48, ptr %v49, ptr %v50, ptr %v51)
  store %"kg.adt.Option[usize]" %v52, ptr %l26
  %v53 = getelementptr %"kg.adt.Option[usize]", ptr %l26, i32 0, i32 0
  %v54 = load i32, ptr %v53
  %v55 = icmp eq i32 %v54, 0
  store i1 %v55, ptr %l27
  %v56 = load i1, ptr %l27
  br i1 %v56, label %b5, label %b6
b4:
  %v57 = call ptr @rt_unit()
  store ptr %v57, ptr %l13
  %v58 = load ptr, ptr %l13
  call void @rt_release(ptr %v58)
  br label %b3
b5:
  %v59 = getelementptr %"kg.adt.Option[usize]", ptr %l26, i32 0, i32 1
  %v60 = load i64, ptr %v59
  store i64 %v60, ptr %l28
  %v61 = load i64, ptr %l28
  store i64 %v61, ptr %l29
  br label %b6
b6:
  %v62 = load i1, ptr %l27
  br i1 %v62, label %b7, label %b8
b7:
  %v63 = load ptr, ptr %l0
  %v64 = call ptr @rt_field(ptr %v63, i64 3)
  store ptr %v64, ptr %l32
  %v65 = load ptr, ptr %l32
  store ptr %v65, ptr %l31
  %v66 = load ptr, ptr %l0
  %v67 = call ptr @rt_field(ptr %v66, i64 4)
  store ptr %v67, ptr %l34
  %v68 = load ptr, ptr %l34
  store ptr %v68, ptr %l33
  %v69 = load ptr, ptr %l33
  %v70 = call i64 @"std/map.gapOf"(ptr %v69)
  store i64 %v70, ptr %l35
  %v71 = load ptr, ptr %l34
  call void @rt_release(ptr %v71)
  %v72 = load ptr, ptr %l0
  %v73 = call ptr @rt_field(ptr %v72, i64 2)
  store ptr %v73, ptr %l36
  %v74 = load ptr, ptr %l36
  %v75 = load i64, ptr %l29
  %v76 = call ptr @rt_int(i64 %v75, i32 64)
  %v77 = call ptr @rt_index(ptr %v74, ptr %v76, ptr @.p33)
  call void @rt_release(ptr %v76)
  store ptr %v77, ptr %l37
  %v78 = load ptr, ptr %l36
  call void @rt_release(ptr %v78)
  store i64 2, ptr %l38
  %v79 = load ptr, ptr %l37
  %v80 = call i64 @rt_int_val(ptr %v79)
  %v81 = load i64, ptr %l38
  %v82 = call { i64, i1 } @llvm.usub.with.overflow.i64(i64 %v80, i64 %v81)
  %v83 = extractvalue { i64, i1 } %v82, 0
  %v84 = extractvalue { i64, i1 } %v82, 1
  br i1 %v84, label %ovf87, label %ovfok88
ovf87:
  call void @rt_panic(ptr @.p34)
  unreachable
ovfok88:
  store i64 %v83, ptr %l39
  %v85 = load ptr, ptr %l37
  call void @rt_release(ptr %v85)
  %v86 = load ptr, ptr %l31
  %v87 = load i64, ptr %l35
  %v88 = load i64, ptr %l39
  %v89 = call i64 @"std/map.seqPosition"(ptr %v86, i64 %v87, i64 %v88)
  store i64 %v89, ptr %l40
  %v90 = load ptr, ptr %l32
  call void @rt_release(ptr %v90)
  call void @llvm.memset.p0.i64(ptr %l41, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %v91 = getelementptr %"kg.adt.Option[usize]", ptr %l41, i32 0, i32 0
  store i32 0, ptr %v91
  %v92 = getelementptr %"kg.adt.Option[usize]", ptr %l41, i32 0, i32 1
  %v93 = load i64, ptr %l40
  store i64 %v93, ptr %v92
  %v94 = load ptr, ptr %l6
  store ptr null, ptr %l6
  call void @rt_drop(ptr %v94)
  %v95 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v95)
  %v96 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v96)
  %v97 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v97)
  %v98 = load %"kg.adt.Option[usize]", ptr %l41
  ret %"kg.adt.Option[usize]" %v98
b8:
  %v99 = call ptr @rt_unit()
  store ptr %v99, ptr %l43
  %v100 = load ptr, ptr %l43
  store ptr %v100, ptr %l30
  br label %b9
b9:
  %v101 = load ptr, ptr %l30
  call void @rt_drop(ptr %v101)
  call void @llvm.memset.p0.i64(ptr %l44, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %v102 = getelementptr %"kg.adt.Option[usize]", ptr %l44, i32 0, i32 0
  store i32 1, ptr %v102
  %v103 = load ptr, ptr %l6
  store ptr null, ptr %l6
  call void @rt_drop(ptr %v103)
  %v104 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v104)
  %v105 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v105)
  %v106 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v106)
  %v107 = load %"kg.adt.Option[usize]", ptr %l44
  ret %"kg.adt.Option[usize]" %v107
b10:
  %v108 = call ptr @rt_unit()
  store ptr %v108, ptr %l42
  %v109 = load ptr, ptr %l42
  call void @rt_release(ptr %v109)
  br label %b9
}
define %"kg.adt.Option[usize]" @"std/map.Map.position[golden.Point,String]"(ptr %p0, %"kg.agg.golden.Point" %p1, ptr %p2, ptr %p3, ptr %p4, ptr %p5) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l1, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = alloca ptr
  store ptr null, ptr %l3
  %l4 = alloca ptr
  store ptr null, ptr %l4
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %l6 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l6, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l7 = alloca ptr
  store ptr null, ptr %l7
  %l8 = alloca i64
  store i64 0, ptr %l8
  %l9 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l9, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l10 = alloca ptr
  store ptr null, ptr %l10
  %l11 = alloca ptr
  store ptr null, ptr %l11
  %l12 = alloca ptr
  store ptr null, ptr %l12
  %l13 = alloca ptr
  store ptr null, ptr %l13
  %l14 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l14, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l15 = alloca ptr
  store ptr null, ptr %l15
  %l16 = alloca ptr
  store ptr null, ptr %l16
  %l17 = alloca ptr
  store ptr null, ptr %l17
  %l18 = alloca ptr
  store ptr null, ptr %l18
  %l19 = alloca ptr
  store ptr null, ptr %l19
  store ptr %p0, ptr %l0
  store %"kg.agg.golden.Point" %p1, ptr %l1
  store ptr %p2, ptr %l2
  store ptr %p3, ptr %l3
  store ptr %p4, ptr %l4
  store ptr %p5, ptr %l5
  br label %b0
b0:
  call void @llvm.memcpy.p0.p0.i64(ptr %l6, ptr %l1, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %v1 = load ptr, ptr %l3
  store ptr %v1, ptr %l7
  %v2 = load ptr, ptr %l7
  %v3 = call ptr @rt_retain(ptr %v2)
  store ptr %v3, ptr %l15
  %v4 = load %"kg.agg.golden.Point", ptr %l6
  %v5 = load ptr, ptr %l15
  %v6 = call i64 @"std/prelude.hash[golden.Point]"(%"kg.agg.golden.Point" %v4, ptr %v5)
  store i64 %v6, ptr %l8
  call void @llvm.memcpy.p0.p0.i64(ptr %l9, ptr %l1, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %v7 = load ptr, ptr %l2
  store ptr %v7, ptr %l10
  %v8 = load ptr, ptr %l3
  store ptr %v8, ptr %l11
  %v9 = load ptr, ptr %l2
  store ptr %v9, ptr %l12
  %v10 = load ptr, ptr %l3
  store ptr %v10, ptr %l13
  %v11 = load ptr, ptr %l10
  %v12 = call ptr @rt_retain(ptr %v11)
  store ptr %v12, ptr %l16
  %v13 = load ptr, ptr %l11
  %v14 = call ptr @rt_retain(ptr %v13)
  store ptr %v14, ptr %l17
  %v15 = load ptr, ptr %l12
  %v16 = call ptr @rt_retain(ptr %v15)
  store ptr %v16, ptr %l18
  %v17 = load ptr, ptr %l13
  %v18 = call ptr @rt_retain(ptr %v17)
  store ptr %v18, ptr %l19
  %v19 = load ptr, ptr %l0
  %v20 = load i64, ptr %l8
  %v21 = load %"kg.agg.golden.Point", ptr %l9
  %v22 = load ptr, ptr %l16
  %v23 = load ptr, ptr %l17
  %v24 = load ptr, ptr %l18
  %v25 = load ptr, ptr %l19
  %v26 = call %"kg.adt.Option[usize]" @"std/map.Map.positionHashed[golden.Point,String]"(ptr %v19, i64 %v20, %"kg.agg.golden.Point" %v21, ptr %v22, ptr %v23, ptr %v24, ptr %v25)
  store %"kg.adt.Option[usize]" %v26, ptr %l14
  %v27 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v27)
  %v28 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v28)
  %v29 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v29)
  %v30 = load ptr, ptr %l2
  store ptr null, ptr %l2
  call void @rt_drop(ptr %v30)
  %v31 = load %"kg.adt.Option[usize]", ptr %l14
  ret %"kg.adt.Option[usize]" %v31
}
define %"kg.adt.Option[usize]" @"std/map.Map.findSlot[golden.Point,String]"(ptr %p0, ptr %p1, %"kg.agg.golden.Point" %p2, ptr %p3, ptr %p4, ptr %p5, ptr %p6) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l2, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l3 = alloca ptr
  store ptr null, ptr %l3
  %l4 = alloca ptr
  store ptr null, ptr %l4
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = alloca ptr
  store ptr null, ptr %l7
  %l8 = alloca i64
  store i64 0, ptr %l8
  %l9 = alloca i64
  store i64 0, ptr %l9
  %l10 = alloca i64
  store i64 0, ptr %l10
  %l11 = alloca i64
  store i64 0, ptr %l11
  %l12 = alloca ptr
  store ptr null, ptr %l12
  %l13 = alloca i64
  store i64 0, ptr %l13
  %l14 = alloca ptr
  store ptr null, ptr %l14
  %l15 = alloca ptr
  store ptr null, ptr %l15
  %l16 = alloca i64
  store i64 0, ptr %l16
  %l17 = alloca i64
  store i64 0, ptr %l17
  %l18 = alloca i1
  store i1 0, ptr %l18
  %l19 = alloca ptr
  store ptr null, ptr %l19
  %l20 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l20, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l21 = alloca ptr
  store ptr null, ptr %l21
  %l22 = alloca ptr
  store ptr null, ptr %l22
  %l23 = alloca i1
  store i1 0, ptr %l23
  %l24 = alloca i64
  store i64 0, ptr %l24
  %l25 = alloca i1
  store i1 0, ptr %l25
  %l26 = alloca i1
  store i1 0, ptr %l26
  %l27 = alloca ptr
  store ptr null, ptr %l27
  %l28 = alloca ptr
  store ptr null, ptr %l28
  %l29 = alloca ptr
  store ptr null, ptr %l29
  %l30 = alloca ptr
  store ptr null, ptr %l30
  %l31 = alloca ptr
  store ptr null, ptr %l31
  %l32 = alloca ptr
  store ptr null, ptr %l32
  %l33 = alloca i64
  store i64 0, ptr %l33
  %l34 = alloca i64
  store i64 0, ptr %l34
  %l35 = alloca i64
  store i64 0, ptr %l35
  %l36 = alloca i64
  store i64 0, ptr %l36
  %l37 = alloca ptr
  store ptr null, ptr %l37
  %l38 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l38, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l39 = alloca i1
  store i1 0, ptr %l39
  %l40 = alloca ptr
  store ptr null, ptr %l40
  %l41 = alloca ptr
  store ptr null, ptr %l41
  %l42 = alloca i64
  store i64 0, ptr %l42
  %l43 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l43, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %l44 = alloca ptr
  store ptr null, ptr %l44
  %l45 = alloca ptr
  store ptr null, ptr %l45
  %l46 = alloca i64
  store i64 0, ptr %l46
  %l47 = alloca i64
  store i64 0, ptr %l47
  %l48 = alloca i64
  store i64 0, ptr %l48
  %l49 = alloca ptr
  store ptr null, ptr %l49
  %l50 = alloca ptr
  store ptr null, ptr %l50
  %l51 = alloca i64
  store i64 0, ptr %l51
  %l52 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l52, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l53 = alloca %"kg.adt.Option[usize]"
  call void @llvm.memset.p0.i64(ptr %l53, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  store ptr %p0, ptr %l0
  store ptr %p1, ptr %l1
  store %"kg.agg.golden.Point" %p2, ptr %l2
  store ptr %p3, ptr %l3
  store ptr %p4, ptr %l4
  store ptr %p5, ptr %l5
  store ptr %p6, ptr %l6
  br label %b0
b0:
  %v1 = load ptr, ptr %l0
  %v2 = call ptr @rt_field(ptr %v1, i64 2)
  store ptr %v2, ptr %l7
  %v3 = load ptr, ptr %l7
  %v4 = call i64 @rt_d_array_len(ptr %v3)
  store i64 %v4, ptr %l8
  %v5 = load ptr, ptr %l7
  call void @rt_release(ptr %v5)
  store i64 1, ptr %l9
  %v6 = load i64, ptr %l8
  %v7 = load i64, ptr %l9
  %v8 = call { i64, i1 } @llvm.usub.with.overflow.i64(i64 %v6, i64 %v7)
  %v9 = extractvalue { i64, i1 } %v8, 0
  %v10 = extractvalue { i64, i1 } %v8, 1
  br i1 %v10, label %ovf12, label %ovfok13
ovf12:
  call void @rt_panic(ptr @.p34)
  unreachable
ovfok13:
  store i64 %v9, ptr %l10
  %v11 = load i64, ptr %l10
  store i64 %v11, ptr %l11
  %v12 = load ptr, ptr %l1
  store ptr %v12, ptr %l12
  %v13 = load ptr, ptr %l12
  %v14 = call i64 @rt_int_val(ptr %v13)
  store i64 %v14, ptr %l13
  br label %b1
b1:
  br label %b2
b2:
  %v15 = load ptr, ptr %l0
  %v16 = call ptr @rt_field(ptr %v15, i64 2)
  store ptr %v16, ptr %l14
  %v17 = load ptr, ptr %l14
  %v18 = load i64, ptr %l13
  %v19 = call ptr @rt_int(i64 %v18, i32 64)
  %v20 = call ptr @rt_index(ptr %v17, ptr %v19, ptr @.p35)
  call void @rt_release(ptr %v19)
  store ptr %v20, ptr %l15
  %v21 = load ptr, ptr %l14
  call void @rt_release(ptr %v21)
  %v22 = load ptr, ptr %l15
  %v23 = call i64 @rt_int_val(ptr %v22)
  call void @rt_release(ptr %v22)
  store i64 %v23, ptr %l16
  store i64 0, ptr %l17
  %v24 = load i64, ptr %l16
  %v25 = load i64, ptr %l17
  %v26 = icmp eq i64 %v24, %v25
  store i1 %v26, ptr %l18
  %v27 = load i1, ptr %l18
  br i1 %v27, label %b5, label %b6
b3:
  br label %b1
b4:
  unreachable
b5:
  call void @llvm.memset.p0.i64(ptr %l20, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %v28 = getelementptr %"kg.adt.Option[usize]", ptr %l20, i32 0, i32 0
  store i32 1, ptr %v28
  %v29 = load ptr, ptr %l6
  store ptr null, ptr %l6
  call void @rt_drop(ptr %v29)
  %v30 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v30)
  %v31 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v31)
  %v32 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v32)
  %v33 = load ptr, ptr %l1
  store ptr null, ptr %l1
  call void @rt_drop(ptr %v33)
  %v34 = load %"kg.adt.Option[usize]", ptr %l20
  ret %"kg.adt.Option[usize]" %v34
b6:
  %v35 = call ptr @rt_unit()
  store ptr %v35, ptr %l22
  %v36 = load ptr, ptr %l22
  store ptr %v36, ptr %l19
  br label %b7
b7:
  %v37 = load ptr, ptr %l19
  call void @rt_drop(ptr %v37)
  store i64 1, ptr %l24
  %v38 = load i64, ptr %l16
  %v39 = load i64, ptr %l24
  %v40 = icmp ugt i64 %v38, %v39
  store i1 %v40, ptr %l25
  %v41 = load i1, ptr %l25
  br i1 %v41, label %b9, label %b10
b8:
  %v42 = call ptr @rt_unit()
  store ptr %v42, ptr %l21
  %v43 = load ptr, ptr %l21
  call void @rt_release(ptr %v43)
  br label %b7
b9:
  %v44 = load ptr, ptr %l0
  %v45 = call ptr @rt_field(ptr %v44, i64 0)
  store ptr %v45, ptr %l28
  %v46 = load ptr, ptr %l0
  %v47 = call ptr @rt_field(ptr %v46, i64 3)
  store ptr %v47, ptr %l30
  %v48 = load ptr, ptr %l30
  store ptr %v48, ptr %l29
  %v49 = load ptr, ptr %l0
  %v50 = call ptr @rt_field(ptr %v49, i64 4)
  store ptr %v50, ptr %l32
  %v51 = load ptr, ptr %l32
  store ptr %v51, ptr %l31
  %v52 = load ptr, ptr %l31
  %v53 = call i64 @"std/map.gapOf"(ptr %v52)
  store i64 %v53, ptr %l33
  %v54 = load ptr, ptr %l32
  call void @rt_release(ptr %v54)
  store i64 2, ptr %l34
  %v55 = load i64, ptr %l16
  %v56 = load i64, ptr %l34
  %v57 = call { i64, i1 } @llvm.usub.with.overflow.i64(i64 %v55, i64 %v56)
  %v58 = extractvalue { i64, i1 } %v57, 0
  %v59 = extractvalue { i64, i1 } %v57, 1
  br i1 %v59, label %ovf63, label %ovfok64
ovf63:
  call void @rt_panic(ptr @.p34)
  unreachable
ovfok64:
  store i64 %v58, ptr %l35
  %v60 = load ptr, ptr %l29
  %v61 = load i64, ptr %l33
  %v62 = load i64, ptr %l35
  %v63 = call i64 @"std/map.seqPosition"(ptr %v60, i64 %v61, i64 %v62)
  store i64 %v63, ptr %l36
  %v64 = load ptr, ptr %l30
  call void @rt_release(ptr %v64)
  %v65 = load ptr, ptr %l28
  %v66 = load i64, ptr %l36
  %v67 = call ptr @rt_int(i64 %v66, i32 64)
  %v68 = call ptr @rt_index(ptr %v65, ptr %v67, ptr @.p36)
  call void @rt_release(ptr %v67)
  store ptr %v68, ptr %l37
  %v69 = load ptr, ptr %l28
  call void @rt_release(ptr %v69)
  %v70 = load ptr, ptr %l37
  store ptr %v70, ptr %l27
  call void @llvm.memcpy.p0.p0.i64(ptr %l38, ptr %l2, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %v71 = load ptr, ptr %l3
  store ptr %v71, ptr %l40
  %v72 = load ptr, ptr %l27
  call void @"kg.unbox.golden.Point"(ptr %l52, ptr %v72)
  %v73 = load %"kg.agg.golden.Point", ptr %l52
  %v74 = load %"kg.agg.golden.Point", ptr %l38
  %v75 = call i1 @"golden.Point.equals"(%"kg.agg.golden.Point" %v73, %"kg.agg.golden.Point" %v74)
  store i1 %v75, ptr %l39
  %v76 = load ptr, ptr %l37
  call void @rt_release(ptr %v76)
  %v77 = load i1, ptr %l39
  store i1 %v77, ptr %l23
  br label %b11
b10:
  store i1 0, ptr %l26
  %v78 = load i1, ptr %l26
  store i1 %v78, ptr %l23
  br label %b11
b11:
  %v79 = load i1, ptr %l23
  br i1 %v79, label %b12, label %b13
b12:
  %v80 = load i64, ptr %l13
  store i64 %v80, ptr %l42
  %v81 = load i64, ptr %l42
  store i64 %v81, ptr %l51
  call void @llvm.memset.p0.i64(ptr %l43, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %v82 = getelementptr %"kg.adt.Option[usize]", ptr %l43, i32 0, i32 0
  store i32 0, ptr %v82
  %v83 = getelementptr %"kg.adt.Option[usize]", ptr %l43, i32 0, i32 1
  %v84 = load i64, ptr %l51
  store i64 %v84, ptr %v83
  %v85 = load ptr, ptr %l6
  store ptr null, ptr %l6
  call void @rt_drop(ptr %v85)
  %v86 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v86)
  %v87 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v87)
  %v88 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v88)
  %v89 = load ptr, ptr %l1
  store ptr null, ptr %l1
  call void @rt_drop(ptr %v89)
  %v90 = load %"kg.adt.Option[usize]", ptr %l43
  ret %"kg.adt.Option[usize]" %v90
b13:
  %v91 = call ptr @rt_unit()
  store ptr %v91, ptr %l45
  %v92 = load ptr, ptr %l45
  store ptr %v92, ptr %l41
  br label %b14
b14:
  %v93 = load ptr, ptr %l41
  call void @rt_drop(ptr %v93)
  store i64 1, ptr %l46
  %v94 = load i64, ptr %l13
  %v95 = load i64, ptr %l46
  %v96 = call { i64, i1 } @llvm.uadd.with.overflow.i64(i64 %v94, i64 %v95)
  %v97 = extractvalue { i64, i1 } %v96, 0
  %v98 = extractvalue { i64, i1 } %v96, 1
  br i1 %v98, label %ovf104, label %ovfok105
ovf104:
  call void @rt_panic(ptr @.p34)
  unreachable
ovfok105:
  store i64 %v97, ptr %l47
  %v99 = load i64, ptr %l47
  %v100 = load i64, ptr %l11
  %v101 = and i64 %v99, %v100
  store i64 %v101, ptr %l48
  %v102 = load i64, ptr %l48
  store i64 %v102, ptr %l13
  %v103 = call ptr @rt_unit()
  store ptr %v103, ptr %l49
  br label %b3
b15:
  %v104 = call ptr @rt_unit()
  store ptr %v104, ptr %l44
  %v105 = load ptr, ptr %l44
  call void @rt_release(ptr %v105)
  br label %b14
b16:
  %v106 = call ptr @rt_unit()
  store ptr %v106, ptr %l50
  %v107 = load ptr, ptr %l6
  store ptr null, ptr %l6
  call void @rt_drop(ptr %v107)
  %v108 = load ptr, ptr %l5
  store ptr null, ptr %l5
  call void @rt_drop(ptr %v108)
  %v109 = load ptr, ptr %l4
  store ptr null, ptr %l4
  call void @rt_drop(ptr %v109)
  %v110 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v110)
  %v111 = load ptr, ptr %l1
  store ptr null, ptr %l1
  call void @rt_drop(ptr %v111)
  %v112 = load ptr, ptr %l50
  call void @"kg.unbox.Option[usize]"(ptr %l53, ptr %v112)
  %v113 = load ptr, ptr %l50
  call void @rt_release(ptr %v113)
  %v114 = load %"kg.adt.Option[usize]", ptr %l53
  ret %"kg.adt.Option[usize]" %v114
}
define ptr @"golden.Point.equals$bind"(ptr %env, ptr %args) {
  %a0 = call ptr @rt_at(ptr %args, i64 0)
  %a1 = call ptr @rt_at(ptr %args, i64 1)
  %v1 = call ptr @"golden.Point.equals$boxed"(ptr %a0, ptr %a1)
  ret ptr %v1
}
define ptr @"golden.Point.equals$boxed"(ptr %p0, ptr %p1) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = alloca i1
  store i1 0, ptr %l2
  %l3 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l3, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  %l4 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l4, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  store ptr %p0, ptr %l0
  store ptr %p1, ptr %l1
  br label %b0
b0:
  %v1 = load ptr, ptr %l0
  call void @"kg.unbox.golden.Point"(ptr %l3, ptr %v1)
  %v2 = load ptr, ptr %l1
  call void @"kg.unbox.golden.Point"(ptr %l4, ptr %v2)
  %v3 = load %"kg.agg.golden.Point", ptr %l3
  %v4 = load %"kg.agg.golden.Point", ptr %l4
  %v5 = call i1 @"golden.Point.equals"(%"kg.agg.golden.Point" %v3, %"kg.agg.golden.Point" %v4)
  store i1 %v5, ptr %l2
  %v6 = load i1, ptr %l2
  %v7 = zext i1 %v6 to i32
  %v8 = call ptr @rt_bool(i32 %v7)
  ret ptr %v8
}
define ptr @"golden.Point.hash$bind"(ptr %env, ptr %args) {
  %a0 = call ptr @rt_at(ptr %args, i64 0)
  %v1 = call ptr @"golden.Point.hash$boxed"(ptr %a0)
  ret ptr %v1
}
define ptr @"golden.Point.hash$boxed"(ptr %p0) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca i64
  store i64 0, ptr %l1
  %l2 = alloca %"kg.agg.golden.Point"
  call void @llvm.memset.p0.i64(ptr %l2, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Point", ptr null, i32 1) to i64), i1 false)
  store ptr %p0, ptr %l0
  br label %b0
b0:
  %v1 = load ptr, ptr %l0
  call void @"kg.unbox.golden.Point"(ptr %l2, ptr %v1)
  %v2 = load %"kg.agg.golden.Point", ptr %l2
  %v3 = call i64 @"golden.Point.hash"(%"kg.agg.golden.Point" %v2)
  store i64 %v3, ptr %l1
  %v4 = load i64, ptr %l1
  %v5 = call ptr @rt_int(i64 %v4, i32 64)
  ret ptr %v5
}
define void @kg_init_statics() {
  ret void
}
define void @kg_fini_statics() {
  ret void
}
define internal i1 @"kg.eq.golden.Point"(ptr %a, ptr %b) {
entry:
  %v1 = getelementptr %"kg.agg.golden.Point", ptr %a, i32 0, i32 0
  %v2 = getelementptr %"kg.agg.golden.Point", ptr %b, i32 0, i32 0
  %v3 = load i64, ptr %v1
  %v4 = load i64, ptr %v2
  %v5 = icmp eq i64 %v3, %v4
  %v6 = and i1 true, %v5
  %v7 = getelementptr %"kg.agg.golden.Point", ptr %a, i32 0, i32 1
  %v8 = getelementptr %"kg.agg.golden.Point", ptr %b, i32 0, i32 1
  %v9 = load i64, ptr %v7
  %v10 = load i64, ptr %v8
  %v11 = icmp eq i64 %v9, %v10
  %v12 = and i1 %v6, %v11
  ret i1 %v12
}
define internal i64 @"kg.hash.golden.Point"(ptr %a) {
entry:
  %v1 = getelementptr %"kg.agg.golden.Point", ptr %a, i32 0, i32 0
  %v2 = load i64, ptr %v1
  %v3 = call i64 @rt_d_hash_int(i64 %v2, i32 320)
  %v4 = xor i64 -3750763034362895579, %v3
  %v5 = mul i64 %v4, 1099511628211
  %v6 = getelementptr %"kg.agg.golden.Point", ptr %a, i32 0, i32 1
  %v7 = load i64, ptr %v6
  %v8 = call i64 @rt_d_hash_int(i64 %v7, i32 320)
  %v9 = xor i64 %v5, %v8
  %v10 = mul i64 %v9, 1099511628211
  ret i64 %v10
}
define internal void @"kg.unbox.Option[usize]"(ptr %d, ptr %b) {
entry:
  %tp = getelementptr %"kg.adt.Option[usize]", ptr %d, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Option[usize]", ptr %d, i32 0, i32 1
  call void @llvm.memset.p0.i64(ptr %d, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[usize]", ptr null, i32 1) to i64), i1 false)
  %tag = call i32 @rt_variant_index(ptr %b)
  store i32 %tag, ptr %tp
  switch i32 %tag, label %bad [ i32 0, label %c0 i32 1, label %c1 ]
c0:
  %v1 = call ptr @rt_payload(ptr %b, i64 0)
  %v2 = call i64 @rt_int_val(ptr %v1)
  store i64 %v2, ptr %pb
  call void @rt_release(ptr %v1)
  br label %done
c1:
  br label %done
bad:
  unreachable
done:
  ret void
}
define internal ptr @"kg.box.golden.Point"(ptr %v) {
entry:
  %v1 = alloca ptr, i32 2
  %v2 = getelementptr %"kg.agg.golden.Point", ptr %v, i32 0, i32 0
  %v3 = load i64, ptr %v2
  %v4 = call ptr @rt_int(i64 %v3, i32 320)
  %v5 = getelementptr ptr, ptr %v1, i32 0
  store ptr %v4, ptr %v5
  %v6 = getelementptr %"kg.agg.golden.Point", ptr %v, i32 0, i32 1
  %v7 = load i64, ptr %v6
  %v8 = call ptr @rt_int(i64 %v7, i32 320)
  %v9 = getelementptr ptr, ptr %v1, i32 1
  store ptr %v8, ptr %v9
  %r = call ptr @rt_record(ptr @".type.golden.Point", i64 2, ptr %v1)
  ret ptr %r
}
define internal void @"kg.unbox.golden.Point"(ptr %d, ptr %b) {
entry:
  %v1 = call ptr @rt_field(ptr %b, i64 0)
  %v2 = getelementptr %"kg.agg.golden.Point", ptr %d, i32 0, i32 0
  %v3 = call i64 @rt_int_val(ptr %v1)
  store i64 %v3, ptr %v2
  call void @rt_release(ptr %v1)
  %v4 = call ptr @rt_field(ptr %b, i64 1)
  %v5 = getelementptr %"kg.agg.golden.Point", ptr %d, i32 0, i32 1
  %v6 = call i64 @rt_int_val(ptr %v4)
  store i64 %v6, ptr %v5
  call void @rt_release(ptr %v4)
  ret void
}
define internal void @"kg.drop.Option[String]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Option[String]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Option[String]", ptr %v, i32 0, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %done [ i32 0, label %c0 ]
c0:
  %v1 = load i64, ptr %pb
  %v2 = inttoptr i64 %v1 to ptr
  call void @llvm.memset.p0.i64(ptr %v, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[String]", ptr null, i32 1) to i64), i1 false)
  call void @rt_release(ptr %v2)
  br label %done
done:
  ret void
}
define internal void @"kg.drop.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr %v, i32 0, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %done [ i32 0, label %c0 ]
c0:
  %v1 = load i64, ptr %pb
  %v2 = inttoptr i64 %v1 to ptr
  call void @llvm.memset.p0.i64(ptr %v, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[std/map.Map[golden.Point,String],std/prelude.AllocError]", ptr null, i32 1) to i64), i1 false)
  call void @rt_release(ptr %v2)
  br label %done
done:
  ret void
}
define internal void @"kg.unbox.Option[String]"(ptr %d, ptr %b) {
entry:
  %tp = getelementptr %"kg.adt.Option[String]", ptr %d, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Option[String]", ptr %d, i32 0, i32 1
  call void @llvm.memset.p0.i64(ptr %d, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[String]", ptr null, i32 1) to i64), i1 false)
  %tag = call i32 @rt_variant_index(ptr %b)
  store i32 %tag, ptr %tp
  switch i32 %tag, label %bad [ i32 0, label %c0 i32 1, label %c1 ]
c0:
  %v1 = call ptr @rt_payload(ptr %b, i64 0)
  %v2 = ptrtoint ptr %v1 to i64
  store i64 %v2, ptr %pb
  br label %done
c1:
  br label %done
bad:
  unreachable
done:
  ret void
}
define i32 @main(i32 %argc, ptr %argv) {
  call void @rt_abi_check(ptr @.p37)
  call void @kg_init_immortals()
  call void @rt_init(i32 %argc, ptr %argv, i32 6, ptr @rt_types)
  call void @kg_init_statics()
  %r = call ptr @"kigumi.main"()
  call void @kg_fini_statics()
  %c = call i32 @rt_exit_code(ptr %r, ptr null, ptr null, i64 0, i64 0)
  ret i32 %c
}

@.p0 = private constant [8 x i8] c".Option\00"
@.p1 = private constant [5 x i8] c"Some\00"
@.p2 = private constant [5 x i8] c"None\00"
@.p3 = private constant [8 x i8] c".Result\00"
@.p4 = private constant [3 x i8] c"Ok\00"
@.p5 = private constant [4 x i8] c"Err\00"
@.p6 = private constant [7 x i8] c".Array\00"
@.p7 = private constant [13 x i8] c"golden.Point\00"
@.p8 = private constant [2 x i8] c"x\00"
@.p9 = private constant [2 x i8] c"y\00"
@.p10 = private constant [12 x i8] c"std/map.Map\00"
@.p11 = private constant [8 x i8] c"keyList\00"
@.p12 = private constant [10 x i8] c"valueList\00"
@.p13 = private constant [6 x i8] c"slots\00"
@.p14 = private constant [8 x i8] c"seqList\00"
@.p15 = private constant [5 x i8] c"meta\00"
@.p16 = private constant [23 x i8] c"std/prelude.AllocError\00"
@.p17 = private constant [12 x i8] c"OutOfMemory\00"
@.p18 = internal global [64 x i8] zeroinitializer, align 16
@.p19 = internal global [64 x i8] zeroinitializer, align 16
@.p20 = internal global [64 x i8] zeroinitializer, align 16
@.p21 = internal global [64 x i8] zeroinitializer, align 16
@.p22 = private constant [6 x i8] c"print\00"
@.p23 = internal global [64 x i8] zeroinitializer, align 16
@.p24 = internal global [64 x i8] zeroinitializer, align 16
@.p25 = internal global [64 x i8] zeroinitializer, align 16
@.p26 = internal global [64 x i8] zeroinitializer, align 16
@.p27 = internal global [64 x i8] zeroinitializer, align 16
@.p28 = internal global [64 x i8] zeroinitializer, align 16
@.p29 = internal global [64 x i8] zeroinitializer, align 16
@.p30 = internal global [64 x i8] zeroinitializer, align 16
@.p31 = internal global [64 x i8] zeroinitializer, align 16
@.p32 = private constant [6 x i8] c"panic\00"
@.p33 = private constant [_ x i8] c"std/map/map.kg:<line>\00"
@.p34 = private constant [17 x i8] c"integer overflow\00"
@.p35 = private constant [_ x i8] c"std/map/map.kg:<line>\00"
@.p36 = private constant [_ x i8] c"std/map/map.kg:<line>\00"
@.p37 = private constant [40 x i8] c"kgabi1:<hash>\00"
