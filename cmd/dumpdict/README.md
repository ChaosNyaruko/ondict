# Dump MDX files into vocab.db
dumpdict -f path/to/dict1.mdx -f path/to/dict2.mdx

# Dump MDD static files into the Ondict cache directory
dumpdict -f path/to/dict.mdd

# Dump MDX/MDD files in specified directories
dumpdict -d dir1 -d dir2

# Mixed
dumpdict -d dir1 -f path/to/dict.mdx -f path/to/dict.mdd
