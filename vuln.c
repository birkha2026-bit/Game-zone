#include <stdio.h>
#include <string.h>
#include <stdlib.h>

void vulnerable_function(char *user_input) {
    char buffer[64];
    strcpy(buffer, user_input);
}

int main(int argc, char *argv[]) {
    if (argc > 1) {
        printf("Processing incoming data packet: %s\n", argv[1]);
        vulnerable_function(argv[1]);
        printf("Buffer processed successfully without immediate crash.\n");
    } else {
        printf("No data received.\n");
    }
    return 0;
}
