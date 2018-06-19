#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <setjmp.h>
#include <signal.h>
#include <math.h>
#include <time.h>
#include "deps/linenoise.h"
#include "deps/pcg_basic.h"

// {{{ value type

typedef struct val {
    short mark;
    short type;
    union {
        struct { struct val* car; struct val* cdr; } lst;
        struct { long n; } num;
        struct { char* name; struct val* cell; } sym;
        struct { char* name; struct val* (*fn)(); } prim;
        struct { struct val* code; struct val* env; } fn;
    } data;
} val;

#define nil ((val*)0)
#define eq(x,y) ((x) == (y))
#define noteq(x,y) ((x) != (y))
#define isnil(x) eq(x, nil)
#define isnotnil(x) noteq(x, nil)
#define type(x) (((x) == nil) ? 0 : ((*(x)).type))
#define typeeq(x,y) (type(x) == (y))
#define typenoteq(x,y) (type(x) != (y))
#define car(x) ((*x).data.lst.car)
#define cdr(x) ((*x).data.lst.cdr)
#define num(x) ((*x).data.num.n)
#define sym_name(x) ((*x).data.sym.name)
#define sym_cell(x) ((*x).data.sym.cell)
#define prim_fn(x) ((*x).data.prim.fn)

#define type_nil     0
#define type_lst     1
#define type_num     2
#define type_sym     3
#define type_prim0   4
#define type_prim1   5
#define type_prim2   6
#define type_prim3   7
#define type_priml   8
#define type_primf   9
#define type_primm   10
#define type_fn 11

// }}} value type

// Garbage collection
val* heap_1; val* heap_2; val* heap; val* heap_end; val* heap_org;
long heap_size = 5000;
long old_heap_used;
int which_heap;
int gc_status_flag = 1;
char *init_file = (char *)NULL;

val* oblist = nil;
val* truth = nil;
val* eof_val = nil;
val* sym_errobj = nil;
val* sym_do = nil;
val* sym_fn = nil;
val* sym_quote = nil;
val* open_files = nil;
val* unbound_marker = nil;

// Reader
#define TOKENMAXSIZE 256
char *read_buffer;
char tok_buffer[TOKENMAXSIZE];

// Error
jmp_buf errjmp;
int errjmp_ok = 0;
int interrupt_ok = 0;

void err(char* message, val* x);
val* intern(char* name);
val* set(val* name, val* value, val* env);
val* envfind(val* var, val* env);
val* eval(val* x, val* env);
val* read_token(int j);
val* read_list();
val* read_str();
val* l_car(val* x);
val* l_cdr(val* x);
val* l_reverse(val* x);

__attribute((noreturn)) void err(char* message, val* x) {
    interrupt_ok = 0;
    if (x) printf("error: %s (see errobj)\n", message);
    else printf("error: %s\n", message);
    if (errjmp_ok == 1) { set(sym_errobj, x, nil); longjmp(errjmp, 1); }
    printf("fatal error in critical code\n");
    exit(1);
}

// {{{ initializers

#define alloc_val(t) register val* z = heap; \
  if (heap >= heap_end) err("ran out of storage", nil);\
  heap = z + 1;\
  (*z).mark = 0;\
  (*z).type = t;

val* cons(val* x, val* y) {
    alloc_val(type_lst);
    (*z).data.lst.car = x;
    (*z).data.lst.cdr = y;
    return z;
}

val* new_num(long n) {
    alloc_val(type_num);
    (*z).data.num.n = n;
    return z;
}

val* new_sym(char* name, val* cell) {
    alloc_val(type_sym);
    (*z).data.sym.name = name;
    (*z).data.sym.cell = cell;
    return z;
}

val* new_prim(int type, char* name, val* (*fn)()) {
    alloc_val(type);
    (*z).data.prim.name = name;
    (*z).data.prim.fn = fn;
    return z;
}

val* new_fn(val* code, val* env) {
    alloc_val(type_fn);
    (*z).data.fn.code = code;
    (*z).data.fn.env = env;
    return z;
}

// }}} initializers
// {{{ gc

char* must_malloc(unsigned long size) {
    char* tmp = (char *)malloc(size);
    if (tmp == (char *)NULL) err("failed to allocate storage from system", nil);
    return tmp;
}

val* gc_relocate(val* x) {
    val* new;
    if isnil(x) return nil;
    if ((*x).mark == 1) return car(x);
    switch type(x) {
    case type_num:
        new = new_num(num(x));
        break;
    case type_lst:
        new = cons(car(x), cdr(x));
        break;
    case type_sym:
        new = new_sym(sym_name(x), sym_cell(x));
        break;
    case type_fn:
        new = new_fn((*x).data.fn.code, (*x).data.fn.env);
        break;
    case type_prim0: case type_prim1: case type_prim2: case type_prim3:
    case type_priml: case type_primf: case type_primm:
        new = new_prim(type(x), (*x).data.prim.name, (*x).data.prim.fn);
        break;
    default:
        err("gc_relocate: bug", nil);
    }
    (*x).mark = 1;
    car(x) = new;
    return new;
}

void gc() {
    errjmp_ok = 0;
    interrupt_ok = 0;

    old_heap_used = heap - heap_org;
    val* h;
    if (which_heap == 1) { h = heap_2; which_heap = 2; }
    else { h = heap_1; which_heap = 1; }
    heap = h;
    heap_org = heap;
    heap_end = heap + heap_size;

    oblist = gc_relocate(oblist);
    eof_val = gc_relocate(eof_val);
    truth = gc_relocate(truth);
    sym_errobj = gc_relocate(sym_errobj);
    sym_do = gc_relocate(sym_do);
    sym_fn = gc_relocate(sym_fn);
    sym_quote = gc_relocate(sym_quote);
    open_files = gc_relocate(open_files);
    unbound_marker = gc_relocate(unbound_marker);

    for (register val* p = h; p < heap; ++p) {
        switch type(p) {
        case type_lst: case type_fn:
            car(p) = gc_relocate(car(p));
            cdr(p) = gc_relocate(cdr(p));
            break;
        case type_sym:
            sym_cell(p) = gc_relocate(sym_cell(p));
            break;
        default:
            break;
        }
    }

    errjmp_ok = 1;
    interrupt_ok = 1;
}

void init_storage() {
    heap_1 = (val*)must_malloc(sizeof(val)*heap_size);
    heap_2 = (val*)must_malloc(sizeof(val)*heap_size);
    heap = heap_1;
    which_heap = 1;
    heap_org = heap;
    heap_end = heap + heap_size;
    unbound_marker = cons(intern("**unbound-marker**"), nil);
    eof_val = cons(intern("eof"), nil);
    truth = intern("t");
    set(truth, truth, nil);
    set(intern("nil"), nil, nil);
    set(intern("let"), intern("let-internal-macro"), nil);
    sym_errobj = intern("errobj");
    set(sym_errobj, nil, nil);
    sym_do = intern("do");
    sym_fn = intern("fn");
    sym_quote = intern("quote");
}

// }}} gc
// {{{ environment

val* intern_try(char* name) {
    for (val* l = oblist; isnotnil(l); l = cdr(l)) {
        if (strcmp(name, sym_name(car(l))) == 0) return car(l);
    }
    return nil;
}

val* intern(char* name) {
    val* sym = intern_try(name);
    if (sym) return sym;
    sym = new_sym(name, unbound_marker);
    oblist = cons(sym, oblist);
    return sym;
}

val* envfind(val* var, val* env) {
    val* frame; val* al; val* fl; val* x;
    for(frame = env; typeeq(frame, type_lst); frame = cdr(frame)) {
        x = car(frame);
        if typenoteq(x, type_lst) err("envfind: damaged frame", x);
        for (fl = car(x), al = cdr(x); typeeq(fl, type_lst); fl = cdr(fl), al = cdr(al)) {
            if (typenoteq(al, type_lst)) err("envfind: too few arguments", x);
            if (eq(car(fl), var)) return al;
        }
    }
    // if isnil(frame) err("envfind: damaged env", env);
    return nil;
}

val* set(val* name, val* value, val* env) {
    if typenoteq(name, type_sym) err("set: arg1 is not symbol", name);
    val* x = envfind(name, env);
    if (isnil(x)) return sym_cell(name) = value;
    return car(x) = value;
}

void set_prim(char* name, int type, val* (*fn)()) {
    set(intern(name), new_prim(type, name, fn), nil);
}

// }}} environment
// {{{ read

void read_skip(char* eoferr) {
    while (*read_buffer != 0 && isspace(*read_buffer)) read_buffer++;
    if (*read_buffer == 0 && eoferr) err(eoferr, nil);
}

val* read_list() {
    printf("1'%s'\n", read_buffer);
    read_skip("read: end of file inside list");
    printf("2'%s'\n", read_buffer);
    if (read_buffer[0] == ')') return nil;
    printf("3'%s'\n", read_buffer);
    val* t = read_str();
    printf("4'%s'\n", read_buffer);
    return cons(t, read_list());
}

val* read_token(int j) {
    val *sym;
    char *p = tok_buffer;
    p[j] = 0;
    if (*p == '-') p += 1;
    {
         int adigit = 0;
         while (isdigit(*p)) { p += 1; adigit = 1; }
         if (*p == '.') {
              p += 1;
              while (isdigit(*p)) {
                   p+=1; adigit = 1;
              }
         }
         if (!adigit) goto a_symbol;
    }
    if (*p == 'e') {
         p += 1;
         if (*p == '-' || *p == '+') p += 1;
         if (!isdigit(*p)) { goto a_symbol; } else { p += 1; }
         while(isdigit(*p)) p += 1;
    }
    if (*p) goto a_symbol;
    return(new_num(atof(tok_buffer)));
 a_symbol:
    sym = intern_try(tok_buffer);
    if (sym) return sym;
    char *name = must_malloc(strlen(tok_buffer)+1);
    strcpy(name, tok_buffer);
    return intern(name);
}

val* read_str() {
    val* l;
    read_skip("read: end of file");
    switch (*read_buffer) {
    case '(':
        l = nil;
        read_buffer++;
        read_skip("read: end of file inside list");
        while (*read_buffer != ')') {
            l = cons(read_str(), l);
            read_skip("read: end of file inside list");
        }
        return l_reverse(l);
    case ')':
        err("read: unexpected close paren", nil);
    case '\'':
        return cons(sym_quote, cons(read_str(), nil));
    }
    int j = 0;
    while (j < TOKENMAXSIZE && *read_buffer != 0 && !isspace(*read_buffer) && !strchr("()'\"", *read_buffer)) {
        tok_buffer[j] = *read_buffer;
        read_buffer++;
        j++;
    }
    tok_buffer[j] = 0;
    if (isdigit(*tok_buffer) || (strchr("+-.", *tok_buffer) && j > 1)) return new_num(atof(tok_buffer));
    val* sym = intern_try(tok_buffer);
    if (sym) return sym;
    char *name = must_malloc(strlen(tok_buffer)+1);
    strcpy(name, tok_buffer);
    return intern(name);
}

val *read_val(char *str) {
    read_buffer = str;
    return read_str();
}

val* readf(FILE* f) {
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    char *fcontent = must_malloc(size+1);
    fcontent[size] = '\0';
    fread(fcontent, 1, size, f);
    if (ferror(f)) err("read: error reading file", nil);
    return read_val(fcontent);
}

// }}} read
// {{{ eval

val* eval_args(val* x, val* env) {
    if (isnil(x)) return nil;
    if (typenoteq(x, type_lst)) err("eval: bad syntax in argument list", x);
    val* t; val* t1; val* t2;
    val* ret = cons(eval(car(x), env), nil);
    for (t1 = ret, t2 = cdr(x); typeeq(t2, type_lst); t1 = t, t2 = cdr(t2)) {
        t = cons(eval(car(t2), env), nil);
        cdr(t1) = t;
    }
    if (isnil(t2)) err("eval: bad syntax in argument list", x);
    return ret;
}

val* eval(val* x, val* env) {
    val* r;
loop:
    switch type(x) {
    case type_sym:
        r = envfind(x, env);
        if (r) return car(r);
        r = sym_cell(x);
        if eq(r, unbound_marker) err("eval: unbound variable", x);
        return r;
    case type_lst:
        r = eval(car(x), env);
        switch type(r) {
        case type_prim0:
            return prim_fn(r)();
        case type_prim1:
            return prim_fn(r)(eval(l_car(cdr(x)), env));
        case type_prim2:
            return prim_fn(r)(eval(l_car(cdr(x)), env), eval(l_car(l_cdr(cdr(x))), env));
        case type_prim3:
            return prim_fn(r)(eval(l_car(cdr(x)), env), eval(l_car(l_cdr(cdr(x))), env), eval(l_car(l_cdr(l_cdr(cdr(x)))), env));
        case type_priml:
            return prim_fn(r)(eval_args(cdr(x), env));
        case type_primf:
            return prim_fn(r)(cdr(x), env);
        case type_primm:
            if (isnil(prim_fn(r)(&x, &env))) return x;
            goto loop;
        case type_fn:
            env = set(eval_args(cdr(x), env), car((*r).data.fn.code), (*r).data.fn.env);
            x = cdr((*r).data.fn.code);
            goto loop;
        case type_sym:
            x = cons(r, cons(cons(sym_quote, cons(x, nil)), nil));
            x = eval(x, nil);
            goto loop;
        default:
            err("eval: bad function", r);
        }
    default:
        return x;
    }
}

// }}} eval
// {{{ print

void print(val* x) {
    val* t;
    switch (type(x)) {
    case type_nil:
        printf("()");
        break;
    case type_lst:
        printf("(");
        print(car(x));
        for (t = cdr(x); typeeq(t, type_lst); t = cdr(t)) {
            printf(" ");
            print(car(t));
        }
        if (isnotnil(t)) { printf(" . "); print(t); }
        printf(")");
        break;
    case type_num:
        printf("%ld", num(x));
        break;
    case type_sym:
        printf("%s", sym_name(x));
        break;
    case type_prim0: case type_prim1: case type_prim2: case type_prim3:
    case type_priml: case type_primf: case type_primm:
        printf("#<prim %d %s>", type(x), (*x).data.prim.name);
        break;
    case type_fn:
        printf("#<fn ");
        print(car((*x).data.fn.code));
        printf(" ");
        print(cdr((*x).data.fn.code));
        printf(">");
        break;
    }
}

void printnl(val* x) {
    print(x);
    printf("\n");
}

// }}} print
// {{{ repl

void repl() {
    int r = setjmp(errjmp);
    if (r == 2) return;
    errjmp_ok = 1;
    interrupt_ok = 1;

    val *x;
    while (1) {
        gc();
        char* line = linenoise("> ");
        if (line == NULL) break;
        x = read_val(line);
        linenoiseHistoryAdd(line);
        linenoiseFree(line);
        printnl(x);
        x = eval(x, nil);
        printnl(x);
    }
}

// }}} repl
// {{{ primitives

val* l_cons(val* x, val* y) {
    return cons(x, y);
}

val* l_car(val* x) {
    if (isnil(x)) return nil;
    return typeeq(x, type_lst) ? car(x) : (err("car: arg1 is not a cell", x), nil);
}

val* l_cdr(val* x) {
    if (isnil(x)) return nil;
    return typeeq(x, type_lst) ? cdr(x) : (err("cdr: arg1 is not a cell", x), nil);
}

val* l_setcarq(val* x, val* y) {
    if (typenoteq(x, type_lst)) err("set-car!: arg1 is not a cell", x);
    car(x) = y;
    return y;
}

val* l_setcdrq(val* x, val* y) {
    if (typenoteq(x, type_lst)) err("set-cdr!: arg1 is not a cell", x);
    cdr(x) = y;
    return y;
}

val* l_error(val* message, val* x) {
    if (typenoteq(message, type_sym)) err("error: arg1 is not a symbol", message);
    err(sym_name(message), x);
    return nil;
}

val* l_setq(val* args, val* env) {
    if (typenoteq(car(args), type_sym)) err("set!: arg1 is not a symbol", car(args));
    return set(car(args), eval(car(cdr(args)), env), env);
}

val* l_add(val* x, val* y) {
    if (typenoteq(x, type_num)) err("add: arg1 is not a number", x);
    if (typenoteq(y, type_num)) err("add: arg2 is not a number", y);
    return new_num(num(x) + num(y));
}

val* l_sub(val* x, val* y) {
    if (typenoteq(x, type_num)) err("sub: arg1 is not a number", x);
    if (typenoteq(y, type_num)) err("sub: arg2 is not a number", y);
    return new_num(num(x) - num(y));
}

val* l_mul(val* x, val* y) {
    if (typenoteq(x, type_num)) err("mul: arg1 is not a number", x);
    if (typenoteq(y, type_num)) err("mul: arg2 is not a number", y);
    return new_num(num(x) * num(y));
}

val* l_div(val* x, val* y) {
    if (typenoteq(x, type_num)) err("div: arg1 is not a number", x);
    if (typenoteq(y, type_num)) err("div: arg2 is not a number", y);
    if (num(y) == 0) err("div: division by zero", y);
    return new_num(num(x) / num(y));
}

val* l_mod(val* x, val* y) {
    if (typenoteq(x, type_num)) err("mod: arg1 is not a number", x);
    if (typenoteq(y, type_num)) err("mod: arg2 is not a number", y);
    if (num(y) == 0) err("mod: division by zero", y);
    return new_num(num(x) % num(y));
}

val* l_greaterp(val* x, val* y) {
    if (typenoteq(x, type_num)) err(">: arg1 is not a number", x);
    if (typenoteq(y, type_num)) err(">: arg2 is not a number", y);
    return num(x) > num(y) ? truth : nil;
}

val* l_lessp(val* x, val* y) {
    if (typenoteq(x, type_num)) err("<: arg1 is not a number", x);
    if (typenoteq(y, type_num)) err("<: arg2 is not a number", y);
    return num(x) < num(y) ? truth : nil;
}

val* l_eqp(val* x, val* y) {
    return eq(x, y) ? truth : nil;
}

val* l_eqlp(val* x, val* y) {
    if (typeeq(x, type_num) && typeeq(y, type_num))
        return num(x) == num(y) ? truth : nil;
    return eq(x, y) ? truth : nil;
}

val* l_read(val* x) {
    return read_val("()");
}

val* l_print(val* x) {
    printnl(x);
    return nil;
}

val* l_eval(val* args, val* env) {
    val* x = l_car(args);
    return eval(x, env);
}

val* l_fn(val* args, val* env) {
    val* body;
    if (isnil(l_cdr(l_cdr(args)))) { body = car(cdr(args)); }
    else { body = cons(sym_do, l_cdr(args)); }

    if (typenoteq(car(args), type_sym)) {
        val* l;
        for (l = car(args); typeeq(l, type_lst); l = cdr(l));
        if isnotnil(l) err("fn: improper formal argument list", car(args));
    }

    return new_fn(cons(car(args), body), env);
}

val* l_if(val** pform, val** penv) {
    val* args = l_cdr(*pform);
    if (isnotnil(eval(l_car(args), *penv))) {
        *pform = l_car(l_cdr(args));
    } else {
        *pform = l_car(l_cdr(l_cdr(args)));
    }
    return truth;
}

val* l_do(val** pform, val** penv) {
    val* l = l_cdr(*pform);
    val* next = l_cdr(l);
    while (isnotnil(next)) {
        eval(l_car(l), *penv);
        l = next;
        next = l_cdr(next);
    }
    *pform = car(l);
    return truth;
}

val* l_quote(val* args, val* env) {
    return l_car(args);
}

val* l_random(val* x) {
    if (typenoteq(x, type_num)) err("random: arg1 is not a number", x);
    return new_num(pcg32_boundedrand(num(x)));
}

val* l_reverse(val* x) {
    val* y = nil;
    while (isnotnil(x)) {
        y = cons(car(x), y);
        x = cdr(x);
    }
    return y;
}

// }}} primitives
// {{{ main

void init() {
    init_storage();
    set_prim("cons", type_prim2, l_cons);
    set_prim("car", type_prim1, l_car);
    set_prim("cdr", type_prim1, l_cdr);
    set_prim("set-car!", type_prim2, l_setcarq);
    set_prim("set-cdr!", type_prim2, l_setcdrq);
    set_prim("set!", type_primf, l_setq);
    set_prim("+", type_prim2, l_add);
    set_prim("-", type_prim2, l_sub);
    set_prim("*", type_prim2, l_mul);
    set_prim("/", type_prim2, l_div);
    set_prim("%", type_prim2, l_mod);
    set_prim(">", type_prim2, l_greaterp);
    set_prim("<", type_prim2, l_lessp);
    set_prim("eq?", type_prim2, l_eqp);
    set_prim("eql?", type_prim2, l_eqlp);
    set_prim("read", type_prim1, l_read);
    set_prim("print", type_prim1, l_print);
    set_prim("eval", type_primf, l_eval);
    set_prim("fn", type_primf, l_fn);
    set_prim("if", type_primm, l_if);
    set_prim("do", type_primm, l_do);
    set_prim("quote", type_primf, l_quote);
    set_prim("error", type_prim2, l_error);
    set_prim("random", type_prim1, l_random);
    set_prim("reverse", type_prim1, l_reverse);
}

int main(int argc,char *argv[]) {
    // Seed rng
    pcg32_srandom(time(NULL) ^ (intptr_t)&printf, (intptr_t)&gc);

    init();
    repl();
}

// }}} main
