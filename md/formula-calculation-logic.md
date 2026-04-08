# Formula Calculation Logic Documentation

## Overview

This document explains the formula calculation engine used in the system. The engine handles dynamic form calculations with support for formulas, tax treatments, and various calculation methods for practitioner remuneration.

---

## Table of Contents

1. [Core Concepts](#core-concepts)
2. [Tax Treatment Types](#tax-treatment-types)
3. [Calculation Methods](#calculation-methods)
4. [Formula Engine](#formula-engine)
5. [Live Calculation](#live-calculation)
6. [BAS (Business Activity Statement) Calculations](#bas-calculations)
7. [P&L (Profit & Loss) Calculations](#pl-calculations)

---

## Core Concepts

### Field Types

- **Manual Fields**: User-entered values (income, expenses, etc.)
- **Computed Fields**: Automatically calculated using formulas
- **Section Fields**: Aggregate totals for field groups (COLLECTION, COST, OTHER_COST)

### Section Types

- **COLLECTION**: Income/revenue fields
- **COST**: Operating expenses
- **OTHER_COST**: Additional deductions (e.g., merchant fees, bank charges)

### Payment Responsibility

For COST fields:
- **CLINIC**: Paid by the clinic
- **OWNER**: Paid by the practitioner/owner

---

## Tax Treatment Types

The system supports four GST calculation methods:

### 1. EXCLUSIVE (Tax-Exclusive)
**User enters NET amount, GST is added on top**

```
Example: User enters $1000
- Net Amount: $1000.00
- GST (10%): $100.00
- Gross Amount: $1100.00
```

**Formula:**
```
GST = Net × 0.10
Gross = Net + GST
```

### 2. INCLUSIVE (Tax-Inclusive)
**User enters GROSS amount, GST is extracted**

```
Example: User enters $1100
- Gross Amount: $1100.00
- GST (1/11): $100.00
- Net Amount: $1000.00
```

**Formula:**
```
GST = Gross ÷ 11
Net = Gross - GST
```

### 3. MANUAL
**User manually enters both NET and GST amounts**

```
Example: User enters Net=$1000, GST=$150
- Net Amount: $1000.00
- GST Amount: $150.00
- Gross Amount: $1150.00
```

**Formula:**
```
Gross = Net + GST (user-provided)
```

### 4. ZERO (GST-Free)
**No GST applies**

```
Example: User enters $1000
- Net Amount: $1000.00
- GST Amount: $0.00
- Gross Amount: $1000.00
```

---

## Calculation Methods

The system supports two practitioner remuneration calculation methods:

### Method 1: SERVICE_FEE (Gross Method)

**Used for**: Clinic-based practitioners with service fee arrangements

**Calculation Steps:**

1. **Calculate Income**
   ```
   Net Income = Sum of all COLLECTION fields (net amounts)
   Income GST = Sum of GST from COLLECTION fields (manual tax only)
   ```

2. **Calculate Expenses**
   ```
   Clinic Expenses = Sum of COST fields where payment_responsibility = "CLINIC"
   Owner Paid = Sum of COST fields where payment_responsibility = "OWNER"
   Expense GST = Sum of GST from clinic expenses
   Other Costs = Sum of OTHER_COST fields
   ```

3. **Calculate Net Amount**
   ```
   Net Amount = Net Income - Total Expenses
   ```

4. **Calculate Service Fee**
   ```
   Service Fee = Net Amount × (Clinic Share % ÷ 100)
   GST on Service Fee = Service Fee × 0.10
   Total Service Fee = Service Fee + GST on Service Fee
   ```

5. **Calculate Remitted Amount**
   ```
   Remitted Amount = Net Amount 
                   - Total Service Fee 
                   - Other Costs 
                   + Owner Paid Expenses 
                   + Income GST
   ```

**Example:**
```
Income: $10,000
Expenses: $2,000 (Clinic: $1,500, Owner: $500)
Other Costs: $200
Clinic Share: 30%

Net Amount = $10,000 - $2,000 = $8,000
Service Fee = $8,000 × 0.30 = $2,400
GST on Service Fee = $2,400 × 0.10 = $240
Total Service Fee = $2,640
Remitted Amount = $8,000 - $2,640 - $200 + $500 = $5,660
```

---

### Method 2: INDEPENDENT_CONTRACTOR (Net Method)

**Used for**: Independent contractors with percentage-based remuneration

**Calculation Steps:**

1. **Calculate Totals**
   ```
   Income = Sum of COLLECTION fields
   Expenses = Sum of COST fields
   Other Costs = Sum of OTHER_COST fields
   ```

2. **Calculate Net Amount**
   ```
   Net Amount = Income - Expenses - Other Costs
   ```

3. **Calculate Remuneration**
   ```
   Total Remuneration = Net Amount × (Owner Share % ÷ 100)
   ```

4. **Calculate Superannuation (if applicable)**
   ```
   Base Remuneration = Total Remuneration
   Super Component = Base Remuneration × (Super % ÷ 100)
   ```

5. **Calculate GST and Invoice Total**
   ```
   GST on Remuneration = Base Remuneration × 0.10
   Invoice Total = Base Remuneration + GST + Super Component
   ```

**Example:**
```
Income: $10,000
Expenses: $2,000
Other Costs: $200
Owner Share: 60%
Super Component: 11%

Net Amount = $10,000 - $2,000 - $200 = $7,800
Total Remuneration = $7,800 × 0.60 = $4,680
Base Remuneration = $4,680
Super Component = $4,680 × 0.11 = $514.80
GST on Remuneration = $4,680 × 0.10 = $468.00
Invoice Total = $4,680 + $468 + $514.80 = $5,662.80
```

---

## Formula Engine

### Formula Structure

Formulas are stored as expression trees with the following node types:

1. **OPERATOR**: Mathematical operations (+, -, ×, ÷)
2. **FIELD**: Reference to another field by key
3. **CONSTANT**: Fixed numeric value
4. **SECTION**: Reference to section total (e.g., SECTION:COLLECTION)
5. **TEXT**: Non-numeric placeholder

### Expression Tree Example

```
Formula: (Income × 0.60) - Expenses

Tree Structure:
    OPERATOR (-)
    ├── OPERATOR (×)
    │   ├── FIELD (income_key)
    │   └── CONSTANT (0.60)
    └── FIELD (expenses_key)
```

### Evaluation Process

1. **Topological Sort**: Formulas are sorted to ensure dependencies are calculated first
2. **Section Aggregation**: Section totals are computed from manual field values
3. **Formula Evaluation**: Each formula is evaluated in dependency order
4. **Tax Application**: Tax treatment is applied to computed field results
5. **Feedback Loop**: Computed values (with tax) feed into dependent formulas

### Dependency Resolution

```
Example:
Field A = 100 (manual)
Field B = A × 2 (computed) → 200
Field C = B + 50 (computed) → 250

Evaluation Order: A → B → C
```

### Tax Treatment in Formulas

When a computed field has a tax type:

- **EXCLUSIVE**: Formula result is NET, GST is added (result × 1.1 = GROSS)
- **INCLUSIVE**: Formula result is GROSS, GST is extracted
- **MANUAL**: Formula result is NET, user-provided GST is added
- **ZERO**: No GST applied

**Important**: Downstream formulas receive the GROSS amount for tax-enabled fields.

---

## Live Calculation

Live calculation provides real-time updates as users enter data in forms.

### Process Flow

1. **Receive Entry Data**: Frontend sends field values with tax information
2. **Normalize Amounts**: Convert entered values to NET amounts based on tax type
   - INCLUSIVE: Extract NET from entered GROSS
   - EXCLUSIVE: Use entered NET as-is
   - MANUAL: Use entered NET, store GST separately
3. **Compute Section Totals**: Aggregate NET amounts by section type
4. **Evaluate Formulas**: Calculate all computed fields in dependency order
5. **Apply Tax Treatment**: Convert computed NET to NET/GST/GROSS breakdown
6. **Return Results**: Send computed field values back to frontend

### Tax Normalization Example

```
Field with INCLUSIVE tax:
User enters: $1100
System interprets:
  - Entered value is GROSS
  - NET = $1100 ÷ 1.1 = $1000
  - GST = $100
  - Use NET ($1000) in formulas

Field with EXCLUSIVE tax:
User enters: $1000
System interprets:
  - Entered value is NET
  - NET = $1000
  - GST = $100 (calculated)
  - GROSS = $1100
  - Use NET ($1000) in formulas
```

### Special Handling: OTHER_COST Section

For OTHER_COST fields, the system uses GROSS amounts in calculations to ensure proper deduction accounting.

---

## BAS (Business Activity Statement) Calculations

BAS reports track GST obligations for tax reporting.

### Key Metrics

1. **G3 - GST-Free Income**: Income without GST
2. **G8 - Taxable Income**: Income subject to GST
3. **1A - GST on Sales**: Total GST collected from customers
4. **G11 - Total Purchases**: All business expenses
5. **1B - GST on Purchases**: GST paid on expenses (claimable)
6. **Net GST Payable**: 1A - 1B (amount owed to ATO)

### Calculation Logic

```
Income Section:
├── G3 (GST-Free) = Sum of COLLECTION fields with BAS category = GST_FREE
├── G8 (Taxable) = Sum of COLLECTION fields with BAS category = TAXABLE
└── 1A (GST on Sales) = GST from taxable income only

Expense Section:
├── Management Fee = Expenses with "management" in account name
├── Laboratory Work = Expenses with "lab" in account name
├── Other Expenses = All other expenses
└── 1B (GST on Purchases) = Total GST from all expenses

Net GST Payable = 1A - 1B
```

### Quarterly Aggregation

BAS reports are grouped by financial quarters:
- Q1: July - September
- Q2: October - December
- Q3: January - March
- Q4: April - June

### BAS Categories

- **TAXABLE**: Subject to 10% GST
- **GST_FREE**: No GST (e.g., medical services)
- **BAS_EXCLUDED**: Not reported on BAS

---

## P&L (Profit & Loss) Calculations

P&L reports show financial performance over time.

### Report Structure

```
Income
├── Total Income (Gross)
└── Total Income (Net)

Cost of Goods Sold (COGS)
├── Direct Costs
└── COGS Total

Gross Profit = Income - COGS

Other Expenses
├── Operating Expenses
└── Other Costs

Net Profit = Gross Profit - Other Expenses
```

### Monthly Summary

Tracks performance by month:
- Income (Net, GST, Gross)
- COGS (Net, GST, Gross)
- Gross Profit
- Other Expenses (Net, GST, Gross)
- Net Profit (Net, Gross)

### Account-Level Detail

Breaks down by chart of accounts:
- Account Code & Name
- Tax Treatment
- Total Amounts (Net, GST, Gross)
- Entry Count

### Financial Year Summary

Aggregates by quarters within financial years:
- Quarter-by-quarter comparison
- Year-to-date totals
- Trend analysis

---

## Rounding Rules

All monetary values are rounded to 2 decimal places using standard rounding:

```go
Round(value × 100) ÷ 100
```

**Example:**
```
$123.456 → $123.46
$123.454 → $123.45
```

---

## Error Handling

### Common Validation Errors

1. **Division by Zero**: Formulas with zero denominators return error
2. **Missing Field**: Referenced field key not found in values
3. **Circular Dependencies**: Detected during topological sort
4. **Invalid Tax Type**: Unsupported tax treatment specified
5. **Missing GST for Manual**: MANUAL tax requires GST amount

### Validation Rules

- All UUIDs must be valid format
- Amounts must be non-negative
- Super component must be 0-100%
- Date ranges must be valid
- Formula expressions must be complete

---

## API Endpoints

### Calculate from Form
```
POST /api/calculate/:form_id
```
Calculates remuneration based on saved form entries.

### Calculate from Entries
```
POST /api/calculate/entries
```
Calculates remuneration from provided entry values (without saving).

### Formula Calculate
```
POST /api/calculate/formula/:form_id
```
Evaluates all computed fields for a form.

### Live Calculate
```
POST /api/calculate/live
```
Real-time calculation as user enters data.

---

## Best Practices

1. **Always use NET amounts** in formula calculations
2. **Apply tax treatment** only at the final step
3. **Validate dependencies** before formula evaluation
4. **Round only final results**, not intermediate calculations
5. **Handle NULL values** gracefully (treat as zero)
6. **Use topological sort** to ensure correct evaluation order
7. **Feedback GROSS amounts** from tax-enabled computed fields

---

## Glossary

- **NET**: Amount excluding GST
- **GST**: Goods and Services Tax (10% in Australia)
- **GROSS**: Amount including GST
- **BAS**: Business Activity Statement (quarterly tax report)
- **P&L**: Profit and Loss statement
- **COGS**: Cost of Goods Sold
- **FY**: Financial Year (July 1 - June 30 in Australia)
- **COA**: Chart of Accounts

---

*Last Updated: 2026-04-08*
