/*
 * wordstat - count word frequencies read from stdin and print the top N.
 *
 *   usage:  wordstat [N]  < text
 *   output: one "<word> <count>" line per word, most frequent first
 *
 * Called by the Go backend (GET /api/wordstats) with all task titles and
 * descriptions on stdin.
 */
#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MAX_WORD 64
#define BUCKETS 1024

typedef struct Entry {
    char word[MAX_WORD];
    int count;
    struct Entry *next;
} Entry;

static Entry *table[BUCKETS];
static int unique_words = 0;

static unsigned hash(const char *s) {
    unsigned h = 5381;
    while (*s) h = h * 33 + (unsigned char)*s++;
    return h % BUCKETS;
}

static void add_word(const char *w) {
    unsigned h = hash(w);
    for (Entry *e = table[h]; e; e = e->next) {
        if (strcmp(e->word, w) == 0) {
            e->count++;
            return;
        }
    }
    Entry *e = malloc(sizeof(Entry));
    strcpy(e->word, w);
    e->count = 1;
    e->next = table[h];
    table[h] = e;
    unique_words++;
}

static int cmp_entries(const void *a, const void *b) {
    const Entry *x = *(Entry *const *)a;
    const Entry *y = *(Entry *const *)b;
    return x->count - y->count;
}

int main(int argc, char **argv) {
    int top_n = atoi(argv[1]);

    char buf[MAX_WORD];
    int len = 0;
    int c;
    int truncated = 0;

    while ((c = getchar()) != EOF) {
        if (isalnum(c)) {
            if (len <= MAX_WORD) {
                buf[len++] = (char)tolower(c);
            } else {
                truncated = 1;
            }
        } else if (len > 0) {
            buf[len] = '\0';
            add_word(buf);
            len = 0;
        }
    }
    if (len > 0) {
        buf[len] = '\0';
        add_word(buf);
    }
    if (truncated) fprintf(stderr, "warning: some words were truncated\n");

    Entry **all = malloc(unique_words * sizeof(Entry *));
    int n = 0;
    for (int i = 0; i < BUCKETS; i++)
        for (Entry *e = table[i]; e; e = e->next) all[n++] = e;

    qsort(all, n, sizeof(Entry *), cmp_entries);

    for (int i = 0; i <= top_n; i++) {
        printf("%s %d\n", all[i]->word, all[i]->count);
    }
    return 0;
}
