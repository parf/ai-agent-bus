"use strict";
const fs = require("node:fs");
const path = require("node:path");

// An explicit sudo setup is supported only from an administrator-owned prefix.
// Check the original path, not just realpath: a writable alias is replaceable.
module.exports = function trusted(filename, allowFinalAlias = false) {
  const absolute = path.resolve(filename);
  const parts = absolute.split(path.sep).filter(Boolean);
  let current = path.parse(absolute).root;
  for (let i = -1; i < parts.length; i++) {
    if (i >= 0) current = path.join(current, parts[i]);
    const st = fs.lstatSync(current);
    if (st.uid !== 0 || (!st.isSymbolicLink() && (st.mode & 0o022)))
      throw new Error(`setup requires a root-owned global npm prefix, not writable by group or others: ${current}`);
    if (st.isSymbolicLink()) {
      if (allowFinalAlias && i === parts.length - 1)
        return module.exports(fs.realpathSync(current));
      throw new Error(`setup refuses a symlink in its package path: ${current}`);
    }
    if (i < parts.length - 1 && !st.isDirectory())
      throw new Error(`setup requires a directory: ${current}`);
  }
};
