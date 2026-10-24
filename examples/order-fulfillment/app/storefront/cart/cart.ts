export interface Line { sku: string; quantity: number; unitPrice: number }
export class Cart {
 private lines: Line[] = [];
 add(line: Line): void { this.lines.push(line); }
 get total(): number { return this.lines.reduce((sum, line) => sum + line.quantity * line.unitPrice, 0); }
 get items(): readonly Line[] { return this.lines; }
}
