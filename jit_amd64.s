#include "funcdata.h"
#include "textflag.h"

// cgocall(*args) with jitcode in the context blob
//   -> runtime·cgocall(jitcode, frame)
//TEXT ·cgocall(SB),NOSPLIT,$16
//        NO_LOCAL_POINTERS
//        LEAQ argframe+0(FP), AX
//        MOVQ AX, 8(SP)
//        MOVQ 8(DX), AX
//        MOVQ AX, 0(SP)
//        CALL runtimecgocall(SB)
//        RET

TEXT ·jitcall(SB),NOSPLIT,$0
        MOVQ jitcode+0(FP), AX
        LEAQ argframe+8(FP), DI
        JMP AX
