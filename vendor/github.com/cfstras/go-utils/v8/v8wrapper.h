#ifndef _V8WRAPPER_H_
#define _V8WRAPPER_H_

#ifdef __cplusplus
extern "C" {
#endif

    // compiles and executes javascript and returns the script return value as string
    char * runv8(const char *jssrc);

#ifdef __cplusplus
}
#endif

#endif