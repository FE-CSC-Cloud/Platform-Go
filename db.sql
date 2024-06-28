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


/*!40101 SET @OLD_CHARACTER_SET_CLIENT = @@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS = @@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION = @@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;

--
-- Database: `DB`
--
CREATE DATABASE IF NOT EXISTS `GOAPI` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
USE `GOAPI`;

-- --------------------------------------------------------

CREATE TABLE `user_tokens`
(
    `token`      varchar(512) NOT NULL,
    `created_at` timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `expires_at` timestamp    NOT NULL,
    `belongs_to` varchar(255) NOT NULL,
    PRIMARY KEY (`token`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE `reset_tokens`
(
    `token`      varchar(512) NOT NULL,
    `created_at` timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `expires_at` timestamp    NOT NULL,
    PRIMARY KEY (`token`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;


-- --------------------------------------------------------

CREATE TABLE `virtual_machines`
(
    `id`               bigint       NOT NULL AUTO_INCREMENT,
    `users_id`         text         NOT NULL,
    `vcenter_id`       varchar(100) NOT NULL,
    `name`             varchar(100) NOT NULL,
    `description`      text         NOT NULL,
    `end_date`         date         NOT NULL,
    `operating_system` varchar(100) NOT NULL,
    `storage`          int          NOT NULL,
    `memory`           mediumint    NOT NULL,
    `ip`               varchar(15)  NOT NULL,
    `deleted_at`       timestamp    NULL DEFAULT NULL,
    `created_at`       text,
    `updated_at`       timestamp    NULL DEFAULT NULL,
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

CREATE TABLE `ip_adresses`
(
    `ip`                 varchar(15) NOT NULL,
    `virtual_machine_id` varchar(10) NULL,
    `created_at`         timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`         timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`ip`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE `sub_domains`
(
    `id`                  INT          NOT NULL AUTO_INCREMENT,
    `virtual_machines_id` INT          NOT NULL,
    `parent_domain`       VARCHAR(255) NOT NULL,
    `subdomain`           VARCHAR(255) NOT NULL,
    `record_type`         VARCHAR(5)   NOT NULL,
    `record_value`        VARCHAR(255) NOT NULL,
    PRIMARY KEY (`id`)
) ENGINE = InnoDB;

-- --------------------------------------------------------

CREATE TABLE `tickets`
(
    `id`           bigint                                   NOT NULL AUTO_INCREMENT,
    `title`        varchar(255)                             NOT NULL,
    `message`      text                                     NOT NULL,
    `user_id`      text                                     NOT NULL,
    `creator_name` varchar(255)                             NOT NULL,
    `status`       enum ('Pending', 'Accepted', 'Rejected') NOT NULL,
    `response`     text                                     NULL,
    `server_id`    bigint                                   NULL,
    `created_at`   timestamp                                NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;

-- --------------------------------------------------------
CREATE TABLE `notifications`
(
    `id`         bigint       NOT NULL AUTO_INCREMENT,
    `title`      varchar(255) NOT NULL,
    `message`    text         NOT NULL,
    `user_id`    varchar(255) NOT NULL,
    `read_notif` tinyint      NOT NULL DEFAULT '0',
    `created_at` timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;

-- --------------------------------------------------------
CREATE TABLE `errors`
(
    `id`         bigint    NOT NULL AUTO_INCREMENT,
    `message`    text      NOT NULL,
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

COMMIT;

/*!40101 SET CHARACTER_SET_CLIENT = @OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS = @OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION = @OLD_COLLATION_CONNECTION */;
