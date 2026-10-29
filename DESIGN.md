# yesh design principles

yesh is intended for small Linux administration scripts: checking services,
deploying a release, checking hosts, or making a backup. A reader should know
Linux commands without having to remember Bash's historical edge cases.

yesh is a language and interpreter for Linux automation, not an interactive
shell. It borrows the natural command-oriented shape of Linux scripts, but it does not
aim to interpret Bash syntax or provide a shell prompt.

The main test is:

> Can a reader understand a script from top to bottom without mentally
> executing Bash?

## Three rules

yesh should not require the programmer to constantly defend against the
language itself.

1. **One construct, one meaning.** Behavior must not change because of a
   syntactic context or an accidental alternate spelling.
2. **No implicit conversions.** A string stays a string, a list stays a list,
   and a command argument stays one argument. Values are not reparsed as code
   and are not subject to implicit word splitting.
3. **Errors are explicit and predictable.** Authors should not need to remember
   exceptions to `set -e` or determine which pipeline stage happened to decide
   the final status.

yesh does not need to reproduce every Bash feature. Many Bash features are
historical compromises, not sound foundations for a new language.

A good yesh script should remain understandable six months later: the author
can open it and immediately see its intent without remembering the tricks that
were once needed to make it work.

These principles are constraints on new syntax and builtins, not just
documentation style. A proposed feature should demonstrate a real Linux
administration scenario that becomes easier to read, review, and repeat.
