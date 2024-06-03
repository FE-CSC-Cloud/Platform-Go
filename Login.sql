-- phpMyAdmin SQL Dump
-- version 5.2.1
-- https://www.phpmyadmin.net/
--
-- Host: db
-- Generation Time: May 29, 2024 at 08:15 AM
-- Server version: 8.3.0
-- PHP Version: 8.2.8

SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
START TRANSACTION;
SET time_zone = "+00:00";


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;

--
-- Database: `Login`
--

-- --------------------------------------------------------

--
-- Table structure for table `user_tokens`
--

CREATE TABLE `user_tokens` (
  `token` varchar(255) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `expires_at` timestamp NOT NULL,
  `belongs_to` varchar(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Table structure for table `virtual_machines`
--

CREATE TABLE `virtual_machines` (
  `id` bigint NOT NULL,
  `users_id` text NOT NULL,
  `vcenter_id` varchar(255) NOT NULL,
  `name` varchar(100) NOT NULL,
  `description` text NOT NULL,
  `end_date` date NOT NULL,
  `operating_system` varchar(100) NOT NULL,
  `storage` int NOT NULL,
  `memory` mediumint NOT NULL,
  `ip` varchar(15) NOT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` text,
  `updated_at` timestamp NULL DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `ip_adresses` (
  `ip` varchar(15) NOT NULL,
  `virtual_machine_id` bigint NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

--
-- Indexes for dumped tables
--

--
-- Indexes for table `user_tokens`
--
ALTER TABLE `user_tokens`
  ADD PRIMARY KEY (`token`);

--
-- Indexes for table `virtual_machines`
--
ALTER TABLE `virtual_machines`
  ADD PRIMARY KEY (`id`);

--
-- Indexes for table `ip_adresses`
--
ALTER TABLE `ip_adresses`
    ADD PRIMARY KEY(`ip`);

--
-- AUTO_INCREMENT for table `virtual_machines`
--
ALTER TABLE `virtual_machines`
  MODIFY `id` bigint NOT NULL AUTO_INCREMENT, AUTO_INCREMENT=1;
COMMIT;

/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
