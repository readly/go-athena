#!/bin/bash

java -Xmx500M -cp "./antlr-4.7.1-complete.jar:$CLASSPATH" org.antlr.v4.Tool -Dlanguage=Go -package internal SqlBase.g4
