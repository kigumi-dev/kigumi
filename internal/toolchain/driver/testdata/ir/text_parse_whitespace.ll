%rt_type = type { ptr, i32, i32, ptr, i32, ptr, ptr, i64, i64 }
%rt_variant = type { ptr, ptr, i32, i32, [64 x i8] }
%"kg.adt.Option[Int]" = type { i32, [1 x i64] }
%"kg.adt.Option[u64]" = type { i32, [1 x i64] }
%"kg.adt.Option[i128]" = type { i32, [2 x i64] }
declare void @llvm.memcpy.p0.p0.i64(ptr, ptr, i64, i1)
declare void @llvm.memset.p0.i64(ptr, i8, i64, i1)
declare void @rt_init(i32, ptr, i32, ptr)
declare void @rt_abi_check(ptr)
declare i32 @rt_exit_code(ptr, ptr, ptr, i64, i64)
declare ptr @rt_unit()
declare ptr @rt_int(i64, i32)
declare ptr @rt_builtin(ptr, i32, ptr)
declare ptr @rt_std_id(i32, i32, ptr)
declare ptr @rt_variant(ptr, i64, ptr)
declare void @rt_const_num(ptr, i32, i32, i64, double)
declare ptr @rt_box_int128(i64, i64, i32)
declare ptr @rt_interp(i32, ptr)
declare void @rt_drop(ptr)
declare void @rt_release(ptr)
declare i64 @rt_d_len(ptr)
@".type.universe.Option" = global %rt_type { ptr @.p0, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Option.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Option.Some" = global %rt_variant { ptr @.p1, ptr @".type.universe.Option", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Option.None" = global %rt_variant { ptr @.p2, ptr @".type.universe.Option", i32 1, i32 0, [64 x i8] zeroinitializer }
@".type.universe.Option.variants" = global [2 x ptr] [ptr @".variant.universe.Option.Some", ptr @".variant.universe.Option.None"]
@".type.universe.Result" = global %rt_type { ptr @.p3, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Result.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Result.Ok" = global %rt_variant { ptr @.p4, ptr @".type.universe.Result", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Result.Err" = global %rt_variant { ptr @.p5, ptr @".type.universe.Result", i32 1, i32 1, [64 x i8] zeroinitializer }
@".type.universe.Result.variants" = global [2 x ptr] [ptr @".variant.universe.Result.Ok", ptr @".variant.universe.Result.Err"]
@".type.std/prelude.AllocError" = global %rt_type { ptr @.p6, i32 2, i32 0, ptr null, i32 1, ptr @".type.std/prelude.AllocError.variants", ptr null, i64 0, i64 0 }
@".variant.std/prelude.AllocError.OutOfMemory" = global %rt_variant { ptr @.p7, ptr @".type.std/prelude.AllocError", i32 0, i32 0, [64 x i8] zeroinitializer }
@".type.std/prelude.AllocError.variants" = global [1 x ptr] [ptr @".variant.std/prelude.AllocError.OutOfMemory"]
@rt_types = global [3 x ptr] [ptr @".type.universe.Option", ptr @".type.universe.Result", ptr @".type.std/prelude.AllocError"]
define ptr @"kigumi.main"() {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = alloca ptr
  store ptr null, ptr %l3
  %l4 = alloca ptr
  store ptr null, ptr %l4
  %l5 = alloca i64
  store i64 0, ptr %l5
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = alloca ptr
  store ptr null, ptr %l7
  %l8 = alloca i64
  store i64 0, ptr %l8
  %l9 = alloca ptr
  store ptr null, ptr %l9
  %l10 = alloca ptr
  store ptr null, ptr %l10
  %l11 = alloca ptr
  store ptr null, ptr %l11
  %l12 = alloca ptr
  store ptr null, ptr %l12
  %l13 = alloca ptr
  store ptr null, ptr %l13
  %l14 = alloca ptr
  store ptr null, ptr %l14
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
  %l20 = alloca ptr
  store ptr null, ptr %l20
  %l21 = alloca ptr
  store ptr null, ptr %l21
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
  %l33 = alloca ptr
  store ptr null, ptr %l33
  %l34 = alloca ptr
  store ptr null, ptr %l34
  %l35 = alloca ptr
  store ptr null, ptr %l35
  %l36 = alloca ptr
  store ptr null, ptr %l36
  %l37 = alloca ptr
  store ptr null, ptr %l37
  %l38 = alloca ptr
  store ptr null, ptr %l38
  %l39 = alloca ptr
  store ptr null, ptr %l39
  %l40 = alloca ptr
  store ptr null, ptr %l40
  %l41 = alloca ptr
  store ptr null, ptr %l41
  %l42 = alloca ptr
  store ptr null, ptr %l42
  %l43 = alloca ptr
  store ptr null, ptr %l43
  %l44 = alloca ptr
  store ptr null, ptr %l44
  %l45 = alloca ptr
  store ptr null, ptr %l45
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
  %l51 = alloca ptr
  store ptr null, ptr %l51
  %l52 = alloca ptr
  store ptr null, ptr %l52
  %l53 = alloca i64
  store i64 0, ptr %l53
  %l54 = alloca ptr
  store ptr null, ptr %l54
  %l55 = alloca ptr
  store ptr null, ptr %l55
  %l56 = alloca ptr
  store ptr null, ptr %l56
  %l57 = alloca ptr
  store ptr null, ptr %l57
  %l58 = alloca i64
  store i64 0, ptr %l58
  %l59 = alloca ptr
  store ptr null, ptr %l59
  %l60 = alloca ptr
  store ptr null, ptr %l60
  %l61 = alloca ptr
  store ptr null, ptr %l61
  %l62 = alloca ptr
  store ptr null, ptr %l62
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
  %l71 = alloca ptr
  store ptr null, ptr %l71
  %l72 = alloca ptr
  store ptr null, ptr %l72
  %l73 = alloca %"kg.adt.Option[Int]"
  call void @llvm.memset.p0.i64(ptr %l73, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[Int]", ptr null, i32 1) to i64), i1 false)
  %l74 = alloca %"kg.adt.Option[Int]"
  call void @llvm.memset.p0.i64(ptr %l74, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[Int]", ptr null, i32 1) to i64), i1 false)
  %l75 = alloca %"kg.adt.Option[Int]"
  call void @llvm.memset.p0.i64(ptr %l75, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[Int]", ptr null, i32 1) to i64), i1 false)
  %l76 = alloca %"kg.adt.Option[Int]"
  call void @llvm.memset.p0.i64(ptr %l76, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[Int]", ptr null, i32 1) to i64), i1 false)
  %l77 = alloca %"kg.adt.Option[Int]"
  call void @llvm.memset.p0.i64(ptr %l77, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[Int]", ptr null, i32 1) to i64), i1 false)
  %l78 = alloca %"kg.adt.Option[Int]"
  call void @llvm.memset.p0.i64(ptr %l78, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[Int]", ptr null, i32 1) to i64), i1 false)
  %l79 = alloca %"kg.adt.Option[Int]"
  call void @llvm.memset.p0.i64(ptr %l79, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[Int]", ptr null, i32 1) to i64), i1 false)
  %l80 = alloca %"kg.adt.Option[u64]"
  call void @llvm.memset.p0.i64(ptr %l80, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[u64]", ptr null, i32 1) to i64), i1 false)
  %l81 = alloca %"kg.adt.Option[u64]"
  call void @llvm.memset.p0.i64(ptr %l81, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[u64]", ptr null, i32 1) to i64), i1 false)
  %l82 = alloca %"kg.adt.Option[u64]"
  call void @llvm.memset.p0.i64(ptr %l82, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[u64]", ptr null, i32 1) to i64), i1 false)
  %l83 = alloca %"kg.adt.Option[u64]"
  call void @llvm.memset.p0.i64(ptr %l83, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[u64]", ptr null, i32 1) to i64), i1 false)
  %l84 = alloca %"kg.adt.Option[i128]"
  call void @llvm.memset.p0.i64(ptr %l84, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[i128]", ptr null, i32 1) to i64), i1 false)
  %l85 = alloca %"kg.adt.Option[i128]"
  call void @llvm.memset.p0.i64(ptr %l85, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[i128]", ptr null, i32 1) to i64), i1 false)
  %v1 = alloca [64 x i8], align 16
  %v2 = alloca [64 x i8], align 16
  %v3 = alloca ptr, i32 7
  %v4 = alloca ptr, i32 1
  %v5 = alloca ptr, i32 1
  %v6 = alloca ptr, i32 1
  %v7 = alloca ptr, i32 1
  %v8 = alloca ptr, i32 1
  %v9 = alloca ptr, i32 1
  %v10 = alloca ptr, i32 1
  %v11 = alloca ptr, i32 1
  %v12 = alloca ptr, i32 14
  %v13 = alloca ptr, i32 1
  %v14 = alloca ptr, i32 1
  %v15 = alloca ptr, i32 1
  %v16 = alloca ptr, i32 4
  %v17 = alloca ptr, i32 1
  %v18 = alloca ptr, i32 1
  %v19 = alloca ptr, i32 1
  %v20 = alloca ptr, i32 4
  %v21 = alloca ptr, i32 1
  %v22 = alloca %"kg.adt.Option[i128]"
  %v23 = alloca ptr, i32 1
  %v24 = alloca %"kg.adt.Option[i128]"
  %v25 = alloca ptr, i32 1
  %v26 = alloca ptr, i32 4
  %v27 = alloca ptr, i32 1
  br label %b0
b0:
  store ptr @.p8, ptr %l1
  %v28 = load ptr, ptr %l1
  %v29 = call ptr @"std/prelude.String.trim"(ptr %v28)
  store ptr %v29, ptr %l2
  %v30 = load ptr, ptr %l1
  call void @rt_release(ptr %v30)
  store ptr @.p9, ptr %l3
  %v31 = load ptr, ptr %l3
  %v32 = call ptr @"std/prelude.String.trim"(ptr %v31)
  store ptr %v32, ptr %l4
  %v33 = load ptr, ptr %l3
  call void @rt_release(ptr %v33)
  %v34 = load ptr, ptr %l4
  %v35 = call i64 @rt_d_len(ptr %v34)
  store i64 %v35, ptr %l5
  %v36 = load ptr, ptr %l4
  call void @rt_release(ptr %v36)
  store ptr @.p10, ptr %l6
  %v37 = load ptr, ptr %l6
  %v38 = call ptr @"std/prelude.String.trim"(ptr %v37)
  store ptr %v38, ptr %l7
  %v39 = load ptr, ptr %l6
  call void @rt_release(ptr %v39)
  %v40 = load ptr, ptr %l7
  %v41 = call i64 @rt_d_len(ptr %v40)
  store i64 %v41, ptr %l8
  %v42 = load ptr, ptr %l7
  call void @rt_release(ptr %v42)
  %v43 = load ptr, ptr %l2
  %v44 = load i64, ptr %l5
  call void @rt_const_num(ptr %v1, i32 1, i32 64, i64 %v44, double 0.0)
  %v45 = load i64, ptr %l8
  call void @rt_const_num(ptr %v2, i32 1, i32 64, i64 %v45, double 0.0)
  %v46 = getelementptr ptr, ptr %v3, i32 0
  store ptr @.p11, ptr %v46
  %v47 = getelementptr ptr, ptr %v3, i32 1
  store ptr %v43, ptr %v47
  %v48 = getelementptr ptr, ptr %v3, i32 2
  store ptr @.p12, ptr %v48
  %v49 = getelementptr ptr, ptr %v3, i32 3
  store ptr %v1, ptr %v49
  %v50 = getelementptr ptr, ptr %v3, i32 4
  store ptr @.p12, ptr %v50
  %v51 = getelementptr ptr, ptr %v3, i32 5
  store ptr %v2, ptr %v51
  %v52 = getelementptr ptr, ptr %v3, i32 6
  store ptr @.p13, ptr %v52
  %v53 = call ptr @rt_interp(i32 7, ptr %v3)
  call void @rt_release(ptr %v1)
  call void @rt_release(ptr %v2)
  store ptr %v53, ptr %l0
  %v54 = load ptr, ptr %l2
  call void @rt_release(ptr %v54)
  %v55 = load ptr, ptr %l0
  %v56 = getelementptr ptr, ptr %v4, i32 0
  store ptr %v55, ptr %v56
  %v57 = call ptr @rt_builtin(ptr @.p14, i32 1, ptr %v4)
  store ptr %v57, ptr %l9
  %v58 = load ptr, ptr %l0
  call void @rt_release(ptr %v58)
  %v59 = load ptr, ptr %l9
  call void @rt_drop(ptr %v59)
  store ptr @.p15, ptr %l12
  %v60 = load ptr, ptr %l12
  %v61 = call %"kg.adt.Option[Int]" @"std/text.parseInt"(ptr %v60)
  store %"kg.adt.Option[Int]" %v61, ptr %l73
  %v62 = call ptr @"kg.box.Option[Int]"(ptr %l73)
  store ptr %v62, ptr %l13
  %v63 = load ptr, ptr %l13
  store ptr %v63, ptr %l11
  %v64 = load ptr, ptr %l11
  %v65 = getelementptr ptr, ptr %v5, i32 0
  store ptr %v64, ptr %v65
  %v66 = call ptr @rt_std_id(i32 7, i32 1, ptr %v5)
  store ptr %v66, ptr %l14
  %v67 = load ptr, ptr %l13
  call void @rt_release(ptr %v67)
  store ptr @.p16, ptr %l16
  %v68 = load ptr, ptr %l16
  %v69 = call %"kg.adt.Option[Int]" @"std/text.parseInt"(ptr %v68)
  store %"kg.adt.Option[Int]" %v69, ptr %l74
  %v70 = call ptr @"kg.box.Option[Int]"(ptr %l74)
  store ptr %v70, ptr %l17
  %v71 = load ptr, ptr %l17
  store ptr %v71, ptr %l15
  %v72 = load ptr, ptr %l15
  %v73 = getelementptr ptr, ptr %v6, i32 0
  store ptr %v72, ptr %v73
  %v74 = call ptr @rt_std_id(i32 7, i32 1, ptr %v6)
  store ptr %v74, ptr %l18
  %v75 = load ptr, ptr %l17
  call void @rt_release(ptr %v75)
  store ptr @.p17, ptr %l20
  %v76 = load ptr, ptr %l20
  %v77 = call %"kg.adt.Option[Int]" @"std/text.parseInt"(ptr %v76)
  store %"kg.adt.Option[Int]" %v77, ptr %l75
  %v78 = call ptr @"kg.box.Option[Int]"(ptr %l75)
  store ptr %v78, ptr %l21
  %v79 = load ptr, ptr %l21
  store ptr %v79, ptr %l19
  %v80 = load ptr, ptr %l19
  %v81 = getelementptr ptr, ptr %v7, i32 0
  store ptr %v80, ptr %v81
  %v82 = call ptr @rt_std_id(i32 7, i32 1, ptr %v7)
  store ptr %v82, ptr %l22
  %v83 = load ptr, ptr %l21
  call void @rt_release(ptr %v83)
  store ptr @.p18, ptr %l24
  %v84 = load ptr, ptr %l24
  %v85 = call %"kg.adt.Option[Int]" @"std/text.parseInt"(ptr %v84)
  store %"kg.adt.Option[Int]" %v85, ptr %l76
  %v86 = call ptr @"kg.box.Option[Int]"(ptr %l76)
  store ptr %v86, ptr %l25
  %v87 = load ptr, ptr %l25
  store ptr %v87, ptr %l23
  %v88 = load ptr, ptr %l23
  %v89 = getelementptr ptr, ptr %v8, i32 0
  store ptr %v88, ptr %v89
  %v90 = call ptr @rt_std_id(i32 7, i32 1, ptr %v8)
  store ptr %v90, ptr %l26
  %v91 = load ptr, ptr %l25
  call void @rt_release(ptr %v91)
  store ptr @.p19, ptr %l28
  %v92 = load ptr, ptr %l28
  %v93 = call %"kg.adt.Option[Int]" @"std/text.parseInt"(ptr %v92)
  store %"kg.adt.Option[Int]" %v93, ptr %l77
  %v94 = call ptr @"kg.box.Option[Int]"(ptr %l77)
  store ptr %v94, ptr %l29
  %v95 = load ptr, ptr %l29
  store ptr %v95, ptr %l27
  %v96 = load ptr, ptr %l27
  %v97 = getelementptr ptr, ptr %v9, i32 0
  store ptr %v96, ptr %v97
  %v98 = call ptr @rt_std_id(i32 7, i32 1, ptr %v9)
  store ptr %v98, ptr %l30
  %v99 = load ptr, ptr %l29
  call void @rt_release(ptr %v99)
  store ptr @.p20, ptr %l32
  %v100 = load ptr, ptr %l32
  %v101 = call %"kg.adt.Option[Int]" @"std/text.parseInt"(ptr %v100)
  store %"kg.adt.Option[Int]" %v101, ptr %l78
  %v102 = call ptr @"kg.box.Option[Int]"(ptr %l78)
  store ptr %v102, ptr %l33
  %v103 = load ptr, ptr %l33
  store ptr %v103, ptr %l31
  %v104 = load ptr, ptr %l31
  %v105 = getelementptr ptr, ptr %v10, i32 0
  store ptr %v104, ptr %v105
  %v106 = call ptr @rt_std_id(i32 7, i32 1, ptr %v10)
  store ptr %v106, ptr %l34
  %v107 = load ptr, ptr %l33
  call void @rt_release(ptr %v107)
  store ptr @.p21, ptr %l36
  %v108 = load ptr, ptr %l36
  %v109 = call %"kg.adt.Option[Int]" @"std/text.parseInt"(ptr %v108)
  store %"kg.adt.Option[Int]" %v109, ptr %l79
  %v110 = call ptr @"kg.box.Option[Int]"(ptr %l79)
  store ptr %v110, ptr %l37
  %v111 = load ptr, ptr %l37
  store ptr %v111, ptr %l35
  %v112 = load ptr, ptr %l35
  %v113 = getelementptr ptr, ptr %v11, i32 0
  store ptr %v112, ptr %v113
  %v114 = call ptr @rt_std_id(i32 7, i32 1, ptr %v11)
  store ptr %v114, ptr %l38
  %v115 = load ptr, ptr %l37
  call void @rt_release(ptr %v115)
  %v116 = load ptr, ptr %l14
  %v117 = load ptr, ptr %l18
  %v118 = load ptr, ptr %l22
  %v119 = load ptr, ptr %l26
  %v120 = load ptr, ptr %l30
  %v121 = load ptr, ptr %l34
  %v122 = load ptr, ptr %l38
  %v123 = getelementptr ptr, ptr %v12, i32 0
  store ptr @.p22, ptr %v123
  %v124 = getelementptr ptr, ptr %v12, i32 1
  store ptr %v116, ptr %v124
  %v125 = getelementptr ptr, ptr %v12, i32 2
  store ptr @.p23, ptr %v125
  %v126 = getelementptr ptr, ptr %v12, i32 3
  store ptr %v117, ptr %v126
  %v127 = getelementptr ptr, ptr %v12, i32 4
  store ptr @.p23, ptr %v127
  %v128 = getelementptr ptr, ptr %v12, i32 5
  store ptr %v118, ptr %v128
  %v129 = getelementptr ptr, ptr %v12, i32 6
  store ptr @.p23, ptr %v129
  %v130 = getelementptr ptr, ptr %v12, i32 7
  store ptr %v119, ptr %v130
  %v131 = getelementptr ptr, ptr %v12, i32 8
  store ptr @.p23, ptr %v131
  %v132 = getelementptr ptr, ptr %v12, i32 9
  store ptr %v120, ptr %v132
  %v133 = getelementptr ptr, ptr %v12, i32 10
  store ptr @.p23, ptr %v133
  %v134 = getelementptr ptr, ptr %v12, i32 11
  store ptr %v121, ptr %v134
  %v135 = getelementptr ptr, ptr %v12, i32 12
  store ptr @.p23, ptr %v135
  %v136 = getelementptr ptr, ptr %v12, i32 13
  store ptr %v122, ptr %v136
  %v137 = call ptr @rt_interp(i32 14, ptr %v12)
  store ptr %v137, ptr %l10
  %v138 = load ptr, ptr %l14
  call void @rt_release(ptr %v138)
  %v139 = load ptr, ptr %l18
  call void @rt_release(ptr %v139)
  %v140 = load ptr, ptr %l22
  call void @rt_release(ptr %v140)
  %v141 = load ptr, ptr %l26
  call void @rt_release(ptr %v141)
  %v142 = load ptr, ptr %l30
  call void @rt_release(ptr %v142)
  %v143 = load ptr, ptr %l34
  call void @rt_release(ptr %v143)
  %v144 = load ptr, ptr %l38
  call void @rt_release(ptr %v144)
  %v145 = load ptr, ptr %l10
  %v146 = getelementptr ptr, ptr %v13, i32 0
  store ptr %v145, ptr %v146
  %v147 = call ptr @rt_builtin(ptr @.p14, i32 1, ptr %v13)
  store ptr %v147, ptr %l39
  %v148 = load ptr, ptr %l10
  call void @rt_release(ptr %v148)
  %v149 = load ptr, ptr %l39
  call void @rt_drop(ptr %v149)
  store ptr @.p24, ptr %l42
  %v150 = load ptr, ptr %l42
  %v151 = call %"kg.adt.Option[u64]" @"std/text.parseUint#(String)"(ptr %v150)
  store %"kg.adt.Option[u64]" %v151, ptr %l80
  %v152 = call ptr @"kg.box.Option[u64]"(ptr %l80)
  store ptr %v152, ptr %l43
  %v153 = load ptr, ptr %l43
  store ptr %v153, ptr %l41
  %v154 = load ptr, ptr %l41
  %v155 = getelementptr ptr, ptr %v14, i32 0
  store ptr %v154, ptr %v155
  %v156 = call ptr @rt_std_id(i32 7, i32 1, ptr %v14)
  store ptr %v156, ptr %l44
  %v157 = load ptr, ptr %l43
  call void @rt_release(ptr %v157)
  store ptr @.p25, ptr %l46
  %v158 = load ptr, ptr %l46
  %v159 = call %"kg.adt.Option[u64]" @"std/text.parseUint#(String)"(ptr %v158)
  store %"kg.adt.Option[u64]" %v159, ptr %l81
  %v160 = call ptr @"kg.box.Option[u64]"(ptr %l81)
  store ptr %v160, ptr %l47
  %v161 = load ptr, ptr %l47
  store ptr %v161, ptr %l45
  %v162 = load ptr, ptr %l45
  %v163 = getelementptr ptr, ptr %v15, i32 0
  store ptr %v162, ptr %v163
  %v164 = call ptr @rt_std_id(i32 7, i32 1, ptr %v15)
  store ptr %v164, ptr %l48
  %v165 = load ptr, ptr %l47
  call void @rt_release(ptr %v165)
  %v166 = load ptr, ptr %l44
  %v167 = load ptr, ptr %l48
  %v168 = getelementptr ptr, ptr %v16, i32 0
  store ptr @.p26, ptr %v168
  %v169 = getelementptr ptr, ptr %v16, i32 1
  store ptr %v166, ptr %v169
  %v170 = getelementptr ptr, ptr %v16, i32 2
  store ptr @.p23, ptr %v170
  %v171 = getelementptr ptr, ptr %v16, i32 3
  store ptr %v167, ptr %v171
  %v172 = call ptr @rt_interp(i32 4, ptr %v16)
  store ptr %v172, ptr %l40
  %v173 = load ptr, ptr %l44
  call void @rt_release(ptr %v173)
  %v174 = load ptr, ptr %l48
  call void @rt_release(ptr %v174)
  %v175 = load ptr, ptr %l40
  %v176 = getelementptr ptr, ptr %v17, i32 0
  store ptr %v175, ptr %v176
  %v177 = call ptr @rt_builtin(ptr @.p14, i32 1, ptr %v17)
  store ptr %v177, ptr %l49
  %v178 = load ptr, ptr %l40
  call void @rt_release(ptr %v178)
  %v179 = load ptr, ptr %l49
  call void @rt_drop(ptr %v179)
  store ptr @.p27, ptr %l52
  store i64 16, ptr %l53
  %v180 = load ptr, ptr %l52
  %v181 = load i64, ptr %l53
  %v182 = call %"kg.adt.Option[u64]" @"std/text.parseUint#(String,usize)"(ptr %v180, i64 %v181)
  store %"kg.adt.Option[u64]" %v182, ptr %l82
  %v183 = call ptr @"kg.box.Option[u64]"(ptr %l82)
  store ptr %v183, ptr %l54
  %v184 = load ptr, ptr %l54
  store ptr %v184, ptr %l51
  %v185 = load ptr, ptr %l51
  %v186 = getelementptr ptr, ptr %v18, i32 0
  store ptr %v185, ptr %v186
  %v187 = call ptr @rt_std_id(i32 7, i32 1, ptr %v18)
  store ptr %v187, ptr %l55
  %v188 = load ptr, ptr %l54
  call void @rt_release(ptr %v188)
  store ptr @.p28, ptr %l57
  store i64 16, ptr %l58
  %v189 = load ptr, ptr %l57
  %v190 = load i64, ptr %l58
  %v191 = call %"kg.adt.Option[u64]" @"std/text.parseUint#(String,usize)"(ptr %v189, i64 %v190)
  store %"kg.adt.Option[u64]" %v191, ptr %l83
  %v192 = call ptr @"kg.box.Option[u64]"(ptr %l83)
  store ptr %v192, ptr %l59
  %v193 = load ptr, ptr %l59
  store ptr %v193, ptr %l56
  %v194 = load ptr, ptr %l56
  %v195 = getelementptr ptr, ptr %v19, i32 0
  store ptr %v194, ptr %v195
  %v196 = call ptr @rt_std_id(i32 7, i32 1, ptr %v19)
  store ptr %v196, ptr %l60
  %v197 = load ptr, ptr %l59
  call void @rt_release(ptr %v197)
  %v198 = load ptr, ptr %l55
  %v199 = load ptr, ptr %l60
  %v200 = getelementptr ptr, ptr %v20, i32 0
  store ptr @.p29, ptr %v200
  %v201 = getelementptr ptr, ptr %v20, i32 1
  store ptr %v198, ptr %v201
  %v202 = getelementptr ptr, ptr %v20, i32 2
  store ptr @.p23, ptr %v202
  %v203 = getelementptr ptr, ptr %v20, i32 3
  store ptr %v199, ptr %v203
  %v204 = call ptr @rt_interp(i32 4, ptr %v20)
  store ptr %v204, ptr %l50
  %v205 = load ptr, ptr %l55
  call void @rt_release(ptr %v205)
  %v206 = load ptr, ptr %l60
  call void @rt_release(ptr %v206)
  %v207 = load ptr, ptr %l50
  %v208 = getelementptr ptr, ptr %v21, i32 0
  store ptr %v207, ptr %v208
  %v209 = call ptr @rt_builtin(ptr @.p14, i32 1, ptr %v21)
  store ptr %v209, ptr %l61
  %v210 = load ptr, ptr %l50
  call void @rt_release(ptr %v210)
  %v211 = load ptr, ptr %l61
  call void @rt_drop(ptr %v211)
  store ptr @.p30, ptr %l64
  %v212 = load ptr, ptr %l64
  call void @"std/text.parseI128"(ptr %v22, ptr %v212)
  call void @llvm.memcpy.p0.p0.i64(ptr %l84, ptr %v22, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[i128]", ptr null, i32 1) to i64), i1 false)
  %v213 = call ptr @"kg.box.Option[i128]"(ptr %l84)
  store ptr %v213, ptr %l65
  %v214 = load ptr, ptr %l65
  store ptr %v214, ptr %l63
  %v215 = load ptr, ptr %l63
  %v216 = getelementptr ptr, ptr %v23, i32 0
  store ptr %v215, ptr %v216
  %v217 = call ptr @rt_std_id(i32 7, i32 1, ptr %v23)
  store ptr %v217, ptr %l66
  %v218 = load ptr, ptr %l65
  call void @rt_release(ptr %v218)
  store ptr @.p31, ptr %l68
  %v219 = load ptr, ptr %l68
  call void @"std/text.parseI128"(ptr %v24, ptr %v219)
  call void @llvm.memcpy.p0.p0.i64(ptr %l85, ptr %v24, i64 ptrtoint (ptr getelementptr (%"kg.adt.Option[i128]", ptr null, i32 1) to i64), i1 false)
  %v220 = call ptr @"kg.box.Option[i128]"(ptr %l85)
  store ptr %v220, ptr %l69
  %v221 = load ptr, ptr %l69
  store ptr %v221, ptr %l67
  %v222 = load ptr, ptr %l67
  %v223 = getelementptr ptr, ptr %v25, i32 0
  store ptr %v222, ptr %v223
  %v224 = call ptr @rt_std_id(i32 7, i32 1, ptr %v25)
  store ptr %v224, ptr %l70
  %v225 = load ptr, ptr %l69
  call void @rt_release(ptr %v225)
  %v226 = load ptr, ptr %l66
  %v227 = load ptr, ptr %l70
  %v228 = getelementptr ptr, ptr %v26, i32 0
  store ptr @.p32, ptr %v228
  %v229 = getelementptr ptr, ptr %v26, i32 1
  store ptr %v226, ptr %v229
  %v230 = getelementptr ptr, ptr %v26, i32 2
  store ptr @.p23, ptr %v230
  %v231 = getelementptr ptr, ptr %v26, i32 3
  store ptr %v227, ptr %v231
  %v232 = call ptr @rt_interp(i32 4, ptr %v26)
  store ptr %v232, ptr %l62
  %v233 = load ptr, ptr %l66
  call void @rt_release(ptr %v233)
  %v234 = load ptr, ptr %l70
  call void @rt_release(ptr %v234)
  %v235 = load ptr, ptr %l62
  %v236 = getelementptr ptr, ptr %v27, i32 0
  store ptr %v235, ptr %v236
  %v237 = call ptr @rt_builtin(ptr @.p14, i32 1, ptr %v27)
  store ptr %v237, ptr %l71
  %v238 = load ptr, ptr %l62
  call void @rt_release(ptr %v238)
  %v239 = load ptr, ptr %l71
  call void @rt_drop(ptr %v239)
  %v240 = call ptr @rt_unit()
  store ptr %v240, ptr %l72
  %v241 = load ptr, ptr %l72
  ret ptr %v241
}
define void @kg_init_statics() {
  ret void
}
define void @kg_fini_statics() {
  ret void
}
define internal ptr @"kg.box.Option[Int]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Option[Int]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Option[Int]", ptr %v, i32 0, i32 1
  %arr = alloca ptr, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %bad [ i32 0, label %c0 i32 1, label %c1 ]
c0:
  %v1 = load i64, ptr %pb
  %v2 = call ptr @rt_int(i64 %v1, i32 320)
  %v3 = getelementptr ptr, ptr %arr, i32 0
  store ptr %v2, ptr %v3
  %v4 = call ptr @rt_variant(ptr @".variant.universe.Option.Some", i64 1, ptr %arr)
  ret ptr %v4
c1:
  %v5 = call ptr @rt_variant(ptr @".variant.universe.Option.None", i64 0, ptr %arr)
  ret ptr %v5
bad:
  unreachable
}
define internal ptr @"kg.box.Option[u64]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Option[u64]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Option[u64]", ptr %v, i32 0, i32 1
  %arr = alloca ptr, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %bad [ i32 0, label %c0 i32 1, label %c1 ]
c0:
  %v1 = load i64, ptr %pb
  %v2 = call ptr @rt_int(i64 %v1, i32 64)
  %v3 = getelementptr ptr, ptr %arr, i32 0
  store ptr %v2, ptr %v3
  %v4 = call ptr @rt_variant(ptr @".variant.universe.Option.Some", i64 1, ptr %arr)
  ret ptr %v4
c1:
  %v5 = call ptr @rt_variant(ptr @".variant.universe.Option.None", i64 0, ptr %arr)
  ret ptr %v5
bad:
  unreachable
}
define internal ptr @"kg.box.Option[i128]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Option[i128]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Option[i128]", ptr %v, i32 0, i32 1
  %arr = alloca ptr, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %bad [ i32 0, label %c0 i32 1, label %c1 ]
c0:
  %v1 = load i128, ptr %pb, align 4
  %v2 = trunc i128 %v1 to i64
  %v3 = lshr i128 %v1, 64
  %v4 = trunc i128 %v3 to i64
  %v5 = call ptr @rt_box_int128(i64 %v2, i64 %v4, i32 384)
  %v6 = getelementptr ptr, ptr %arr, i32 0
  store ptr %v5, ptr %v6
  %v7 = call ptr @rt_variant(ptr @".variant.universe.Option.Some", i64 1, ptr %arr)
  ret ptr %v7
c1:
  %v8 = call ptr @rt_variant(ptr @".variant.universe.Option.None", i64 0, ptr %arr)
  ret ptr %v8
bad:
  unreachable
}
define i32 @main(i32 %argc, ptr %argv) {
  call void @rt_abi_check(ptr @.p33)
  call void @kg_init_immortals()
  call void @rt_init(i32 %argc, ptr %argv, i32 3, ptr @rt_types)
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
@.p6 = private constant [23 x i8] c"std/prelude.AllocError\00"
@.p7 = private constant [12 x i8] c"OutOfMemory\00"
@.p8 = internal global [64 x i8] zeroinitializer, align 16
@.p9 = internal global [64 x i8] zeroinitializer, align 16
@.p10 = internal global [64 x i8] zeroinitializer, align 16
@.p11 = internal global [64 x i8] zeroinitializer, align 16
@.p12 = internal global [64 x i8] zeroinitializer, align 16
@.p13 = internal global [64 x i8] zeroinitializer, align 16
@.p14 = private constant [6 x i8] c"print\00"
@.p15 = internal global [64 x i8] zeroinitializer, align 16
@.p16 = internal global [64 x i8] zeroinitializer, align 16
@.p17 = internal global [64 x i8] zeroinitializer, align 16
@.p18 = internal global [64 x i8] zeroinitializer, align 16
@.p19 = internal global [64 x i8] zeroinitializer, align 16
@.p20 = internal global [64 x i8] zeroinitializer, align 16
@.p21 = internal global [64 x i8] zeroinitializer, align 16
@.p22 = internal global [64 x i8] zeroinitializer, align 16
@.p23 = internal global [64 x i8] zeroinitializer, align 16
@.p24 = internal global [64 x i8] zeroinitializer, align 16
@.p25 = internal global [64 x i8] zeroinitializer, align 16
@.p26 = internal global [64 x i8] zeroinitializer, align 16
@.p27 = internal global [64 x i8] zeroinitializer, align 16
@.p28 = internal global [64 x i8] zeroinitializer, align 16
@.p29 = internal global [64 x i8] zeroinitializer, align 16
@.p30 = internal global [64 x i8] zeroinitializer, align 16
@.p31 = internal global [64 x i8] zeroinitializer, align 16
@.p32 = internal global [64 x i8] zeroinitializer, align 16
@.p33 = private constant [40 x i8] c"kgabi1:<hash>\00"
