-- Migration: Products and Transactions
-- E-commerce related tables

-- ================================
-- PRODUCT_CATEGORIES
-- ================================
CREATE TABLE IF NOT EXISTS product_categories (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  image_url TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ================================
-- PRODUCTS
-- ================================
CREATE TABLE IF NOT EXISTS products (
  id SERIAL PRIMARY KEY,
  owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  category_id INTEGER REFERENCES product_categories(id) ON DELETE SET NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  image_url TEXT,
  stock INTEGER DEFAULT 0,
  price BIGINT DEFAULT 0,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ================================
-- TRANSACTIONS
-- ================================
CREATE TABLE IF NOT EXISTS transactions (
  id SERIAL PRIMARY KEY,
  buyer_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  qty INTEGER NOT NULL CHECK (qty > 0),
  total_price BIGINT NOT NULL CHECK (total_price >= 0),
  status VARCHAR(50) NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ================================
-- BALANCES
-- ================================
CREATE TABLE IF NOT EXISTS balances (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  amount BIGINT DEFAULT 0,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ================================
-- BALANCE_LOGS
-- ================================
CREATE TABLE IF NOT EXISTS balance_logs (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  transaction_id INTEGER REFERENCES transactions(id) ON DELETE SET NULL,
  amount BIGINT NOT NULL,
  type VARCHAR(20) NOT NULL, -- 'in' atau 'out'
  description TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ================================
-- INDEXES
-- ================================
CREATE INDEX IF NOT EXISTS idx_transactions_buyer_user_id ON transactions(buyer_user_id);
CREATE INDEX IF NOT EXISTS idx_products_owner_user_id ON products(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_products_category_id ON products(category_id);
