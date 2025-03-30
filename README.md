# Rag Chat #

Rag Chat 

### Features ###

* Super performant, developed in Go
* No database required; it uses SQLite as an embedded database
* For vector embeddings the vector database Qdrant is used so need to be instaled
* As a pdf parser, MuPDF library need to be instaled

### How do I get set up? ###

Clone the main branch, compile with the latest Go version, set the environment variables in the .env file, and you are good to go.

Because SQLite is used as an embedded database engine, which is a C library, if you want to use the Rag-Chat backend on another system, you have to compile it using CGo with the following command:

 CGO_ENABLED=1 CC=musl-gcc go build --ldflags '-linkmode=external -extldflags=-static'

After the initial start, the migration will be automatically executed, and the SQLite database will be created in the same folder as the binary file. 