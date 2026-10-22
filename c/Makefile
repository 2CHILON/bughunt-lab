# Build:   make            -> bin/wordstat (debug symbols, -O0)
# Sanitize: make asan      -> bin/wordstat-asan (AddressSanitizer + UBSan)
# Valgrind: make valgrind  (Linux/WSL)
CC ?= gcc
CFLAGS = -g -O0 -Wall -Wextra

all: bin/wordstat

bin:
	mkdir -p bin

bin/wordstat: wordstat.c | bin
	$(CC) $(CFLAGS) -o $@ wordstat.c

asan: | bin
	$(CC) $(CFLAGS) -fsanitize=address,undefined -fno-omit-frame-pointer -o bin/wordstat-asan wordstat.c

valgrind: bin/wordstat
	echo "go go go the quick brown fox go" | valgrind --leak-check=full ./bin/wordstat 3

clean:
	rm -rf bin
