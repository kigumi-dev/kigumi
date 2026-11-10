%rt_type = type { ptr, i32, i32, ptr, i32, ptr, ptr, i64, i64 }
%rt_variant = type { ptr, ptr, i32, i32, [64 x i8] }
declare { i64, i1 } @llvm.sadd.with.overflow.i64(i64, i64)
declare void @rt_init(i32, ptr, i32, ptr)
declare void @rt_abi_check(ptr)
declare i32 @rt_exit_code(ptr, ptr, ptr, i64, i64)
declare ptr @rt_unit()
declare ptr @rt_int(i64, i32)
declare i1 @rt_truth(ptr)
declare ptr @rt_at(ptr, i64)
declare ptr @rt_builtin(ptr, i32, ptr)
declare ptr @rt_std_id(i32, i32, ptr)
declare void @rt_const_num(ptr, i32, i32, i64, double)
declare ptr @rt_payload(ptr, i64)
declare ptr @rt_isvariant(ptr, ptr)
declare void @rt_cell_init(ptr, ptr)
declare ptr @rt_interp(i32, ptr)
declare void @rt_drop(ptr)
declare void @rt_release(ptr)
declare void @rt_panic(ptr)
declare i64 @rt_int_val(ptr)
@".type.universe.Option" = global %rt_type { ptr @.p0, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Option.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Option.Some" = global %rt_variant { ptr @.p1, ptr @".type.universe.Option", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Option.None" = global %rt_variant { ptr @.p2, ptr @".type.universe.Option", i32 1, i32 0, [64 x i8] zeroinitializer }
@".type.universe.Option.variants" = global [2 x ptr] [ptr @".variant.universe.Option.Some", ptr @".variant.universe.Option.None"]
@".type.universe.Result" = global %rt_type { ptr @.p3, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Result.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Result.Ok" = global %rt_variant { ptr @.p4, ptr @".type.universe.Result", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Result.Err" = global %rt_variant { ptr @.p5, ptr @".type.universe.Result", i32 1, i32 1, [64 x i8] zeroinitializer }
@".type.universe.Result.variants" = global [2 x ptr] [ptr @".variant.universe.Result.Ok", ptr @".variant.universe.Result.Err"]
@".type.universe.Array" = global %rt_type { ptr @.p6, i32 0, i32 0, ptr null, i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.std/map.Map" = global %rt_type { ptr @.p7, i32 4, i32 5, ptr @".type.std/map.Map.fields", i32 0, ptr null, ptr null, i64 0, i64 31 }
@".type.std/map.Map.fields" = global [5 x ptr] [ptr @.p8, ptr @.p9, ptr @.p10, ptr @.p11, ptr @.p12]
@".type.std/prelude.AllocError" = global %rt_type { ptr @.p13, i32 2, i32 0, ptr null, i32 1, ptr @".type.std/prelude.AllocError.variants", ptr null, i64 0, i64 0 }
@".variant.std/prelude.AllocError.OutOfMemory" = global %rt_variant { ptr @.p14, ptr @".type.std/prelude.AllocError", i32 0, i32 0, [64 x i8] zeroinitializer }
@".type.std/prelude.AllocError.variants" = global [1 x ptr] [ptr @".variant.std/prelude.AllocError.OutOfMemory"]
@rt_types = global [5 x ptr] [ptr @".type.universe.Option", ptr @".type.universe.Result", ptr @".type.universe.Array", ptr @".type.std/map.Map", ptr @".type.std/prelude.AllocError"]
@"main.closure1$adapter.fnitem" = internal global [64 x i8] zeroinitializer, align 16
@"main.closure2$adapter.fnitem" = internal global [64 x i8] zeroinitializer, align 16
define ptr @"main.closure1"(ptr %p0) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = alloca i64
  store i64 0, ptr %l3
  %l4 = alloca i64
  store i64 0, ptr %l4
  %l5 = alloca i64
  store i64 0, ptr %l5
  %l6 = alloca i64
  store i64 0, ptr %l6
  %l7 = alloca i64
  store i64 0, ptr %l7
  store ptr %p0, ptr %l0
  br label %b0
b0:
  %v1 = load ptr, ptr %l0
  %v2 = call ptr @rt_isvariant(ptr %v1, ptr @".variant.universe.Option.Some")
  store ptr %v2, ptr %l1
  %v3 = load ptr, ptr %l1
  %v4 = call i1 @rt_truth(ptr %v3)
  br i1 %v4, label %b1, label %b2
b1:
  %v5 = load ptr, ptr %l0
  %v6 = call ptr @rt_payload(ptr %v5, i64 0)
  store ptr %v6, ptr %l2
  %v7 = load ptr, ptr %l2
  %v8 = call i64 @rt_int_val(ptr %v7)
  call void @rt_release(ptr %v7)
  store i64 %v8, ptr %l3
  br label %b2
b2:
  %v9 = load ptr, ptr %l1
  %v10 = call i1 @rt_truth(ptr %v9)
  br i1 %v10, label %b3, label %b4
b3:
  store i64 1, ptr %l5
  %v11 = load i64, ptr %l3
  %v12 = load i64, ptr %l5
  %v13 = call { i64, i1 } @llvm.sadd.with.overflow.i64(i64 %v11, i64 %v12)
  %v14 = extractvalue { i64, i1 } %v13, 0
  %v15 = extractvalue { i64, i1 } %v13, 1
  br i1 %v15, label %ovf16, label %ovfok17
ovf16:
  call void @rt_panic(ptr @.p15)
  unreachable
ovfok17:
  store i64 %v14, ptr %l6
  %v16 = load i64, ptr %l6
  store i64 %v16, ptr %l4
  br label %b5
b4:
  store i64 1, ptr %l7
  %v17 = load i64, ptr %l7
  store i64 %v17, ptr %l4
  br label %b5
b5:
  %v18 = load ptr, ptr %l0
  store ptr null, ptr %l0
  call void @rt_drop(ptr %v18)
  %v19 = load i64, ptr %l4
  %v20 = call ptr @rt_int(i64 %v19, i32 320)
  ret ptr %v20
}
define ptr @"main.closure2"(ptr %p0) {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = alloca i64
  store i64 0, ptr %l3
  %l4 = alloca i64
  store i64 0, ptr %l4
  %l5 = alloca i64
  store i64 0, ptr %l5
  %l6 = alloca i64
  store i64 0, ptr %l6
  %l7 = alloca i64
  store i64 0, ptr %l7
  store ptr %p0, ptr %l0
  br label %b0
b0:
  %v1 = load ptr, ptr %l0
  %v2 = call ptr @rt_isvariant(ptr %v1, ptr @".variant.universe.Option.Some")
  store ptr %v2, ptr %l1
  %v3 = load ptr, ptr %l1
  %v4 = call i1 @rt_truth(ptr %v3)
  br i1 %v4, label %b1, label %b2
b1:
  %v5 = load ptr, ptr %l0
  %v6 = call ptr @rt_payload(ptr %v5, i64 0)
  store ptr %v6, ptr %l2
  %v7 = load ptr, ptr %l2
  %v8 = call i64 @rt_int_val(ptr %v7)
  call void @rt_release(ptr %v7)
  store i64 %v8, ptr %l3
  br label %b2
b2:
  %v9 = load ptr, ptr %l1
  %v10 = call i1 @rt_truth(ptr %v9)
  br i1 %v10, label %b3, label %b4
b3:
  store i64 10, ptr %l5
  %v11 = load i64, ptr %l3
  %v12 = load i64, ptr %l5
  %v13 = call { i64, i1 } @llvm.sadd.with.overflow.i64(i64 %v11, i64 %v12)
  %v14 = extractvalue { i64, i1 } %v13, 0
  %v15 = extractvalue { i64, i1 } %v13, 1
  br i1 %v15, label %ovf16, label %ovfok17
ovf16:
  call void @rt_panic(ptr @.p15)
  unreachable
ovfok17:
  store i64 %v14, ptr %l6
  %v16 = load i64, ptr %l6
  store i64 %v16, ptr %l4
  br label %b5
b4:
  store i64 10, ptr %l7
  %v17 = load i64, ptr %l7
  store i64 %v17, ptr %l4
  br label %b5
b5:
  %v18 = load ptr, ptr %l0
  store ptr null, ptr %l0
  call void @rt_drop(ptr %v18)
  %v19 = load i64, ptr %l4
  %v20 = call ptr @rt_int(i64 %v19, i32 320)
  ret ptr %v20
}
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
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = alloca i64
  store i64 0, ptr %l7
  %l8 = alloca ptr
  store ptr null, ptr %l8
  %l9 = alloca ptr
  store ptr null, ptr %l9
  %l10 = alloca ptr
  store ptr null, ptr %l10
  %l11 = alloca ptr
  store ptr null, ptr %l11
  %l12 = alloca i64
  store i64 0, ptr %l12
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
  %l30 = alloca i64
  store i64 0, ptr %l30
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
  %v1 = alloca [64 x i8], align 16
  %v2 = alloca ptr, i32 2
  %v3 = alloca ptr, i32 1
  %v4 = alloca [64 x i8], align 16
  %v5 = alloca ptr, i32 1
  %v6 = alloca ptr, i32 1
  %v7 = alloca ptr, i32 2
  %v8 = alloca ptr, i32 1
  %v9 = alloca [64 x i8], align 16
  %v10 = alloca ptr, i32 2
  %v11 = alloca ptr, i32 1
  %v12 = alloca [64 x i8], align 16
  %v13 = alloca ptr, i32 1
  %v14 = alloca ptr, i32 1
  %v15 = alloca ptr, i32 2
  %v16 = alloca ptr, i32 1
  %v17 = alloca [64 x i8], align 16
  %v18 = alloca ptr, i32 2
  %v19 = alloca ptr, i32 1
  br label %b0
b0:
  store ptr @.p16, ptr %l0
  store ptr @.p17, ptr %l1
  %v20 = load ptr, ptr %l0
  %v21 = load ptr, ptr %l1
  %v22 = call ptr @"std/map.empty"(ptr %v20, ptr %v21)
  store ptr %v22, ptr %l2
  %v23 = load ptr, ptr %l2
  store ptr %v23, ptr %l3
  store ptr @.p18, ptr %l5
  %v24 = load ptr, ptr %l5
  store ptr %v24, ptr %l6
  store i64 7, ptr %l7
  store ptr @.p16, ptr %l8
  store ptr @.p17, ptr %l9
  store ptr @.p16, ptr %l10
  store ptr @.p17, ptr %l11
  %v25 = load ptr, ptr %l3
  %v26 = load ptr, ptr %l6
  %v27 = load i64, ptr %l7
  %v28 = load ptr, ptr %l8
  %v29 = load ptr, ptr %l9
  %v30 = load ptr, ptr %l10
  %v31 = load ptr, ptr %l11
  %v32 = call i64 @"std/map.Map.getOr[String,Int]"(ptr %v25, ptr %v26, i64 %v27, ptr %v28, ptr %v29, ptr %v30, ptr %v31)
  store i64 %v32, ptr %l12
  %v33 = load ptr, ptr %l5
  call void @rt_release(ptr %v33)
  %v34 = load i64, ptr %l12
  call void @rt_const_num(ptr %v1, i32 1, i32 320, i64 %v34, double 0.0)
  %v35 = getelementptr ptr, ptr %v2, i32 0
  store ptr @.p19, ptr %v35
  %v36 = getelementptr ptr, ptr %v2, i32 1
  store ptr %v1, ptr %v36
  %v37 = call ptr @rt_interp(i32 2, ptr %v2)
  call void @rt_release(ptr %v1)
  store ptr %v37, ptr %l4
  %v38 = load ptr, ptr %l4
  %v39 = getelementptr ptr, ptr %v3, i32 0
  store ptr %v38, ptr %v39
  %v40 = call ptr @rt_builtin(ptr @.p20, i32 1, ptr %v3)
  store ptr %v40, ptr %l13
  %v41 = load ptr, ptr %l4
  call void @rt_release(ptr %v41)
  %v42 = load ptr, ptr %l13
  call void @rt_drop(ptr %v42)
  %v43 = load ptr, ptr %l3
  store ptr %v43, ptr %l16
  %v44 = load ptr, ptr %l16
  call void @rt_cell_init(ptr %v4, ptr %v44)
  store ptr %v4, ptr %l17
  store ptr @.p18, ptr %l18
  store ptr @"main.closure1$adapter.fnitem", ptr %l19
  store ptr @.p16, ptr %l20
  store ptr @.p17, ptr %l21
  store ptr @.p16, ptr %l22
  store ptr @.p17, ptr %l23
  %v45 = load ptr, ptr %l17
  %v46 = load ptr, ptr %l18
  %v47 = load ptr, ptr %l19
  %v48 = load ptr, ptr %l20
  %v49 = load ptr, ptr %l21
  %v50 = load ptr, ptr %l22
  %v51 = load ptr, ptr %l23
  %v52 = call ptr @"std/map.Map.update"(ptr %v45, ptr %v46, ptr %v47, ptr %v48, ptr %v49, ptr %v50, ptr %v51)
  store ptr %v52, ptr %l24
  %v53 = load ptr, ptr %l17
  %v54 = load ptr, ptr %l24
  store ptr %v54, ptr %l15
  %v55 = load ptr, ptr %l15
  %v56 = getelementptr ptr, ptr %v6, i32 0
  store ptr %v55, ptr %v56
  %v57 = call ptr @rt_std_id(i32 7, i32 1, ptr %v6)
  store ptr %v57, ptr %l25
  %v58 = load ptr, ptr %l24
  call void @rt_release(ptr %v58)
  %v59 = load ptr, ptr %l25
  %v60 = getelementptr ptr, ptr %v7, i32 0
  store ptr @.p21, ptr %v60
  %v61 = getelementptr ptr, ptr %v7, i32 1
  store ptr %v59, ptr %v61
  %v62 = call ptr @rt_interp(i32 2, ptr %v7)
  store ptr %v62, ptr %l14
  %v63 = load ptr, ptr %l25
  call void @rt_release(ptr %v63)
  %v64 = load ptr, ptr %l14
  %v65 = getelementptr ptr, ptr %v8, i32 0
  store ptr %v64, ptr %v65
  %v66 = call ptr @rt_builtin(ptr @.p20, i32 1, ptr %v8)
  store ptr %v66, ptr %l26
  %v67 = load ptr, ptr %l14
  call void @rt_release(ptr %v67)
  %v68 = load ptr, ptr %l26
  call void @rt_drop(ptr %v68)
  store ptr @.p18, ptr %l28
  %v69 = load ptr, ptr %l28
  store ptr %v69, ptr %l29
  store i64 0, ptr %l30
  store ptr @.p16, ptr %l31
  store ptr @.p17, ptr %l32
  store ptr @.p16, ptr %l33
  store ptr @.p17, ptr %l34
  %v70 = load ptr, ptr %l3
  %v71 = load ptr, ptr %l29
  %v72 = load i64, ptr %l30
  %v73 = load ptr, ptr %l31
  %v74 = load ptr, ptr %l32
  %v75 = load ptr, ptr %l33
  %v76 = load ptr, ptr %l34
  %v77 = call i64 @"std/map.Map.getOr[String,Int]"(ptr %v70, ptr %v71, i64 %v72, ptr %v73, ptr %v74, ptr %v75, ptr %v76)
  store i64 %v77, ptr %l35
  %v78 = load ptr, ptr %l28
  call void @rt_release(ptr %v78)
  %v79 = load i64, ptr %l35
  call void @rt_const_num(ptr %v9, i32 1, i32 320, i64 %v79, double 0.0)
  %v80 = getelementptr ptr, ptr %v10, i32 0
  store ptr @.p22, ptr %v80
  %v81 = getelementptr ptr, ptr %v10, i32 1
  store ptr %v9, ptr %v81
  %v82 = call ptr @rt_interp(i32 2, ptr %v10)
  call void @rt_release(ptr %v9)
  store ptr %v82, ptr %l27
  %v83 = load ptr, ptr %l27
  %v84 = getelementptr ptr, ptr %v11, i32 0
  store ptr %v83, ptr %v84
  %v85 = call ptr @rt_builtin(ptr @.p20, i32 1, ptr %v11)
  store ptr %v85, ptr %l36
  %v86 = load ptr, ptr %l27
  call void @rt_release(ptr %v86)
  %v87 = load ptr, ptr %l36
  call void @rt_drop(ptr %v87)
  %v88 = load ptr, ptr %l3
  store ptr %v88, ptr %l39
  %v89 = load ptr, ptr %l39
  call void @rt_cell_init(ptr %v12, ptr %v89)
  store ptr %v12, ptr %l40
  store ptr @.p18, ptr %l41
  store ptr @"main.closure2$adapter.fnitem", ptr %l42
  store ptr @.p16, ptr %l43
  store ptr @.p17, ptr %l44
  store ptr @.p16, ptr %l45
  store ptr @.p17, ptr %l46
  %v90 = load ptr, ptr %l40
  %v91 = load ptr, ptr %l41
  %v92 = load ptr, ptr %l42
  %v93 = load ptr, ptr %l43
  %v94 = load ptr, ptr %l44
  %v95 = load ptr, ptr %l45
  %v96 = load ptr, ptr %l46
  %v97 = call ptr @"std/map.Map.update"(ptr %v90, ptr %v91, ptr %v92, ptr %v93, ptr %v94, ptr %v95, ptr %v96)
  store ptr %v97, ptr %l47
  %v98 = load ptr, ptr %l40
  %v99 = load ptr, ptr %l47
  store ptr %v99, ptr %l38
  %v100 = load ptr, ptr %l38
  %v101 = getelementptr ptr, ptr %v14, i32 0
  store ptr %v100, ptr %v101
  %v102 = call ptr @rt_std_id(i32 7, i32 1, ptr %v14)
  store ptr %v102, ptr %l48
  %v103 = load ptr, ptr %l47
  call void @rt_release(ptr %v103)
  %v104 = load ptr, ptr %l48
  %v105 = getelementptr ptr, ptr %v15, i32 0
  store ptr @.p23, ptr %v105
  %v106 = getelementptr ptr, ptr %v15, i32 1
  store ptr %v104, ptr %v106
  %v107 = call ptr @rt_interp(i32 2, ptr %v15)
  store ptr %v107, ptr %l37
  %v108 = load ptr, ptr %l48
  call void @rt_release(ptr %v108)
  %v109 = load ptr, ptr %l37
  %v110 = getelementptr ptr, ptr %v16, i32 0
  store ptr %v109, ptr %v110
  %v111 = call ptr @rt_builtin(ptr @.p20, i32 1, ptr %v16)
  store ptr %v111, ptr %l49
  %v112 = load ptr, ptr %l37
  call void @rt_release(ptr %v112)
  %v113 = load ptr, ptr %l49
  call void @rt_drop(ptr %v113)
  store ptr @.p18, ptr %l51
  %v114 = load ptr, ptr %l51
  store ptr %v114, ptr %l52
  store i64 0, ptr %l53
  store ptr @.p16, ptr %l54
  store ptr @.p17, ptr %l55
  store ptr @.p16, ptr %l56
  store ptr @.p17, ptr %l57
  %v115 = load ptr, ptr %l3
  %v116 = load ptr, ptr %l52
  %v117 = load i64, ptr %l53
  %v118 = load ptr, ptr %l54
  %v119 = load ptr, ptr %l55
  %v120 = load ptr, ptr %l56
  %v121 = load ptr, ptr %l57
  %v122 = call i64 @"std/map.Map.getOr[String,Int]"(ptr %v115, ptr %v116, i64 %v117, ptr %v118, ptr %v119, ptr %v120, ptr %v121)
  store i64 %v122, ptr %l58
  %v123 = load ptr, ptr %l51
  call void @rt_release(ptr %v123)
  %v124 = load i64, ptr %l58
  call void @rt_const_num(ptr %v17, i32 1, i32 320, i64 %v124, double 0.0)
  %v125 = getelementptr ptr, ptr %v18, i32 0
  store ptr @.p24, ptr %v125
  %v126 = getelementptr ptr, ptr %v18, i32 1
  store ptr %v17, ptr %v126
  %v127 = call ptr @rt_interp(i32 2, ptr %v18)
  call void @rt_release(ptr %v17)
  store ptr %v127, ptr %l50
  %v128 = load ptr, ptr %l50
  %v129 = getelementptr ptr, ptr %v19, i32 0
  store ptr %v128, ptr %v129
  %v130 = call ptr @rt_builtin(ptr @.p20, i32 1, ptr %v19)
  store ptr %v130, ptr %l59
  %v131 = load ptr, ptr %l50
  call void @rt_release(ptr %v131)
  %v132 = load ptr, ptr %l59
  call void @rt_drop(ptr %v132)
  %v133 = load ptr, ptr %l3
  store ptr null, ptr %l3
  call void @rt_drop(ptr %v133)
  %v134 = call ptr @rt_unit()
  store ptr %v134, ptr %l60
  %v135 = load ptr, ptr %l60
  ret ptr %v135
}
define ptr @"main.closure1$adapter"(ptr %env, ptr %args) {
  %a0 = call ptr @rt_at(ptr %args, i64 0)
  %v1 = call ptr @"main.closure1"(ptr %a0)
  ret ptr %v1
}
define ptr @"main.closure2$adapter"(ptr %env, ptr %args) {
  %a0 = call ptr @rt_at(ptr %args, i64 0)
  %v1 = call ptr @"main.closure2"(ptr %a0)
  ret ptr %v1
}
define void @kg_init_statics() {
  ret void
}
define void @kg_fini_statics() {
  ret void
}
define i32 @main(i32 %argc, ptr %argv) {
  call void @rt_abi_check(ptr @.p25)
  call void @kg_init_immortals()
  call void @rt_init(i32 %argc, ptr %argv, i32 5, ptr @rt_types)
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
@.p7 = private constant [12 x i8] c"std/map.Map\00"
@.p8 = private constant [8 x i8] c"keyList\00"
@.p9 = private constant [10 x i8] c"valueList\00"
@.p10 = private constant [6 x i8] c"slots\00"
@.p11 = private constant [8 x i8] c"seqList\00"
@.p12 = private constant [5 x i8] c"meta\00"
@.p13 = private constant [23 x i8] c"std/prelude.AllocError\00"
@.p14 = private constant [12 x i8] c"OutOfMemory\00"
@.p15 = private constant [17 x i8] c"integer overflow\00"
@.p16 = internal global [64 x i8] zeroinitializer, align 16
@.p17 = internal global [64 x i8] zeroinitializer, align 16
@.p18 = internal global [64 x i8] zeroinitializer, align 16
@.p19 = internal global [64 x i8] zeroinitializer, align 16
@.p20 = private constant [6 x i8] c"print\00"
@.p21 = internal global [64 x i8] zeroinitializer, align 16
@.p22 = internal global [64 x i8] zeroinitializer, align 16
@.p23 = internal global [64 x i8] zeroinitializer, align 16
@.p24 = internal global [64 x i8] zeroinitializer, align 16
@.p25 = private constant [40 x i8] c"kgabi1:<hash>\00"
