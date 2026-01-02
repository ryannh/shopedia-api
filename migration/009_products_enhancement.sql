-- Migration: Products & Categories Enhancement
-- Add uuid, soft delete, blocking, and additional fields

-- ================================
-- UPDATE PRODUCT_CATEGORIES TABLE
-- ================================
ALTER TABLE product_categories ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT gen_random_uuid();
ALTER TABLE product_categories ADD COLUMN IF NOT EXISTS slug VARCHAR(255);
ALTER TABLE product_categories ADD COLUMN IF NOT EXISTS icon VARCHAR(255);
ALTER TABLE product_categories ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;
ALTER TABLE product_categories ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP DEFAULT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_categories_uuid ON product_categories(uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_categories_slug ON product_categories(slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_product_categories_deleted_at ON product_categories(deleted_at);
CREATE INDEX IF NOT EXISTS idx_product_categories_is_active ON product_categories(is_active);

-- ================================
-- UPDATE PRODUCTS TABLE
-- ================================
ALTER TABLE products ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT gen_random_uuid();
ALTER TABLE products ADD COLUMN IF NOT EXISTS slug VARCHAR(255);
ALTER TABLE products ADD COLUMN IF NOT EXISTS images TEXT[] DEFAULT '{}';
ALTER TABLE products ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'active'; -- active, blocked
ALTER TABLE products ADD COLUMN IF NOT EXISTS block_reason TEXT;
ALTER TABLE products ADD COLUMN IF NOT EXISTS blocked_at TIMESTAMP;
ALTER TABLE products ADD COLUMN IF NOT EXISTS blocked_by INTEGER REFERENCES users(id);
ALTER TABLE products ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP DEFAULT NULL;

-- Rename name to title if exists (optional - may fail if already done)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'products' AND column_name = 'name') THEN
        ALTER TABLE products RENAME COLUMN name TO title;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_products_uuid ON products(uuid);
CREATE INDEX IF NOT EXISTS idx_products_slug ON products(slug);
CREATE INDEX IF NOT EXISTS idx_products_status ON products(status);
CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);
CREATE INDEX IF NOT EXISTS idx_products_owner_status ON products(owner_user_id, status);

-- ================================
-- SEED: Default Categories
-- ================================
INSERT INTO product_categories (name, slug, icon, description) VALUES
  ('Elektronik', 'elektronik', 'laptop', 'Perangkat elektronik dan gadget'),
  ('Fashion Pria', 'fashion-pria', 'shirt', 'Pakaian dan aksesoris pria'),
  ('Fashion Wanita', 'fashion-wanita', 'dress', 'Pakaian dan aksesoris wanita'),
  ('Makanan & Minuman', 'makanan-minuman', 'utensils', 'Produk makanan dan minuman'),
  ('Kesehatan', 'kesehatan', 'heart-pulse', 'Produk kesehatan dan kecantikan'),
  ('Rumah Tangga', 'rumah-tangga', 'home', 'Peralatan rumah tangga'),
  ('Olahraga', 'olahraga', 'dumbbell', 'Peralatan dan perlengkapan olahraga'),
  ('Otomotif', 'otomotif', 'car', 'Aksesoris dan sparepart kendaraan'),
  ('Hobi', 'hobi', 'gamepad', 'Produk hobi dan koleksi'),
  ('Lainnya', 'lainnya', 'box', 'Kategori lainnya')
ON CONFLICT DO NOTHING;

-- ================================
-- SEED: Sample Products
-- ================================
DO $$
DECLARE
    v_user_id INTEGER;
    v_cat_elektronik INTEGER;
    v_cat_fashion_pria INTEGER;
    v_cat_fashion_wanita INTEGER;
    v_cat_makanan INTEGER;
    v_cat_olahraga INTEGER;
BEGIN
    -- Get first user as owner (or skip if no users)
    SELECT id INTO v_user_id FROM users LIMIT 1;

    IF v_user_id IS NULL THEN
        RAISE NOTICE 'No users found, skipping product seeds';
        RETURN;
    END IF;

    -- Get category IDs
    SELECT id INTO v_cat_elektronik FROM product_categories WHERE slug = 'elektronik';
    SELECT id INTO v_cat_fashion_pria FROM product_categories WHERE slug = 'fashion-pria';
    SELECT id INTO v_cat_fashion_wanita FROM product_categories WHERE slug = 'fashion-wanita';
    SELECT id INTO v_cat_makanan FROM product_categories WHERE slug = 'makanan-minuman';
    SELECT id INTO v_cat_olahraga FROM product_categories WHERE slug = 'olahraga';

    -- Insert sample products
    INSERT INTO products (owner_user_id, category_id, title, slug, description, images, price, stock, status, is_active) VALUES
      (v_user_id, v_cat_elektronik, 'iPhone 15 Pro Max', 'iphone-15-pro-max', 'iPhone 15 Pro Max 256GB Natural Titanium', ARRAY['https://example.com/iphone1.jpg', 'https://example.com/iphone2.jpg'], 21999000, 10, 'active', TRUE),
      (v_user_id, v_cat_elektronik, 'MacBook Pro M3', 'macbook-pro-m3', 'MacBook Pro 14 inch M3 Pro 18GB 512GB', ARRAY['https://example.com/macbook1.jpg'], 32999000, 5, 'active', TRUE),
      (v_user_id, v_cat_elektronik, 'Samsung Galaxy S24 Ultra', 'samsung-galaxy-s24-ultra', 'Samsung Galaxy S24 Ultra 512GB Titanium Black', ARRAY['https://example.com/samsung1.jpg'], 19999000, 15, 'active', TRUE),
      (v_user_id, v_cat_fashion_pria, 'Kemeja Formal Slim Fit', 'kemeja-formal-slim-fit', 'Kemeja formal pria slim fit bahan katun premium', ARRAY['https://example.com/kemeja1.jpg'], 299000, 50, 'active', TRUE),
      (v_user_id, v_cat_fashion_pria, 'Celana Chino Pria', 'celana-chino-pria', 'Celana chino pria casual comfortable fit', ARRAY['https://example.com/celana1.jpg'], 349000, 30, 'active', TRUE),
      (v_user_id, v_cat_fashion_wanita, 'Dress Casual Wanita', 'dress-casual-wanita', 'Dress casual wanita motif bunga elegan', ARRAY['https://example.com/dress1.jpg'], 259000, 25, 'active', TRUE),
      (v_user_id, v_cat_makanan, 'Kopi Arabika Toraja 250g', 'kopi-arabika-toraja', 'Kopi arabika asli Toraja premium roasted', ARRAY['https://example.com/kopi1.jpg'], 85000, 100, 'active', TRUE),
      (v_user_id, v_cat_makanan, 'Cokelat Premium Gift Box', 'cokelat-premium-gift-box', 'Paket cokelat premium untuk hadiah', ARRAY['https://example.com/cokelat1.jpg'], 175000, 40, 'active', TRUE),
      (v_user_id, v_cat_olahraga, 'Sepatu Running Nike', 'sepatu-running-nike', 'Sepatu lari Nike Air Zoom Pegasus', ARRAY['https://example.com/nike1.jpg'], 1899000, 20, 'active', TRUE),
      (v_user_id, v_cat_olahraga, 'Dumbbell Set 20kg', 'dumbbell-set-20kg', 'Set dumbbell adjustable 20kg untuk home gym', ARRAY['https://example.com/dumbbell1.jpg'], 599000, 15, 'active', TRUE)
    ON CONFLICT DO NOTHING;

    RAISE NOTICE 'Sample products seeded successfully';
END $$;
