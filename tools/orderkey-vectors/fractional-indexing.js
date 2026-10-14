/**
 * fractional-indexing@3.2.0 —— 原样取自 Cherry Studio 打包产物中的实现
 * （app.asar 内 node_modules/.pnpm/fractional-indexing@3.2.0/node_modules/fractional-indexing/src/index.js），
 * 仅补上 exports，用作生成测试向量的「真值」参考实现。
 *
 * MIT License, Copyright (c) 2021 Rocicorp LLC.
 * 上游：https://github.com/rocicorp/fractional-indexing
 */

export const BASE_62_DIGITS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";

function midpoint(a, b, digits) {
  const zero = digits[0];
  if (b != null && a >= b) throw new Error(a + " >= " + b);
  if (a.slice(-1) === zero || (b && b.slice(-1) === zero)) throw new Error("trailing zero");
  if (b) {
    let n = 0;
    while ((a[n] || zero) === b[n]) n++;
    if (n > 0) return b.slice(0, n) + midpoint(a.slice(n), b.slice(n), digits);
  }
  const digitA = a ? digits.indexOf(a[0]) : 0;
  const digitB = b != null ? digits.indexOf(b[0]) : digits.length;
  if (digitB - digitA > 1) return digits[Math.round(0.5 * (digitA + digitB))];
  else if (b && b.length > 1) return b.slice(0, 1);
  else return digits[digitA] + midpoint(a.slice(1), null, digits);
}

function validateInteger(int) {
  if (int.length !== getIntegerLength(int[0])) throw new Error("invalid integer part of order key: " + int);
}

function getIntegerLength(head) {
  if (head >= "a" && head <= "z") return head.charCodeAt(0) - "a".charCodeAt(0) + 2;
  else if (head >= "A" && head <= "Z") return "Z".charCodeAt(0) - head.charCodeAt(0) + 2;
  else throw new Error("invalid order key head: " + head);
}

function getIntegerPart(key) {
  const integerPartLength = getIntegerLength(key[0]);
  if (integerPartLength > key.length) throw new Error("invalid order key: " + key);
  return key.slice(0, integerPartLength);
}

function validateOrderKey(key, digits) {
  if (key === "A" + digits[0].repeat(26)) throw new Error("invalid order key: " + key);
  const i = getIntegerPart(key);
  if (key.slice(i.length).slice(-1) === digits[0]) throw new Error("invalid order key: " + key);
}

function incrementInteger(x, digits) {
  validateInteger(x);
  const [head, ...digs] = x.split("");
  let carry = true;
  for (let i = digs.length - 1; carry && i >= 0; i--) {
    const d = digits.indexOf(digs[i]) + 1;
    if (d === digits.length) digs[i] = digits[0];
    else {
      digs[i] = digits[d];
      carry = false;
    }
  }
  if (carry) {
    if (head === "Z") return "a" + digits[0];
    if (head === "z") return null;
    const h = String.fromCharCode(head.charCodeAt(0) + 1);
    if (h > "a") digs.push(digits[0]);
    else digs.pop();
    return h + digs.join("");
  } else return head + digs.join("");
}

function decrementInteger(x, digits) {
  validateInteger(x);
  const [head, ...digs] = x.split("");
  let borrow = true;
  for (let i = digs.length - 1; borrow && i >= 0; i--) {
    const d = digits.indexOf(digs[i]) - 1;
    if (d === -1) digs[i] = digits.slice(-1);
    else {
      digs[i] = digits[d];
      borrow = false;
    }
  }
  if (borrow) {
    if (head === "a") return "Z" + digits.slice(-1);
    if (head === "A") return null;
    const h = String.fromCharCode(head.charCodeAt(0) - 1);
    if (h < "Z") digs.push(digits.slice(-1));
    else digs.pop();
    return h + digs.join("");
  } else return head + digs.join("");
}

export function generateKeyBetween(a, b, digits = BASE_62_DIGITS) {
  if (a != null) validateOrderKey(a, digits);
  if (b != null) validateOrderKey(b, digits);
  if (a != null && b != null && a >= b) throw new Error(a + " >= " + b);
  if (a == null) {
    if (b == null) return "a" + digits[0];
    const ib = getIntegerPart(b);
    const fb = b.slice(ib.length);
    if (ib === "A" + digits[0].repeat(26)) return ib + midpoint("", fb, digits);
    if (ib < b) return ib;
    const res = decrementInteger(ib, digits);
    if (res == null) throw new Error("cannot decrement any more");
    return res;
  }
  if (b == null) {
    const ia = getIntegerPart(a);
    const fa = a.slice(ia.length);
    const i = incrementInteger(ia, digits);
    return i == null ? ia + midpoint(fa, null, digits) : i;
  }
  const ia = getIntegerPart(a);
  const fa = a.slice(ia.length);
  const ib = getIntegerPart(b);
  const fb = b.slice(ib.length);
  if (ia === ib) return ia + midpoint(fa, fb, digits);
  const i = incrementInteger(ia, digits);
  if (i == null) throw new Error("cannot increment any more");
  if (i < b) return i;
  return ia + midpoint(fa, null, digits);
}
